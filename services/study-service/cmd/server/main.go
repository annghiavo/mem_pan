package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"mem_pan/pkg/logger"
	"mem_pan/pkg/middleware"
	"mem_pan/services/study-service/config"
	"mem_pan/services/study-service/doc"
	"mem_pan/services/study-service/internal/authclient"
	"mem_pan/services/study-service/internal/billingclient"
	"mem_pan/services/study-service/internal/deckclient"
	"mem_pan/services/study-service/internal/fsrsopt"
	"mem_pan/services/study-service/internal/gapi"
	"mem_pan/services/study-service/internal/moderationclient"
	"mem_pan/services/study-service/internal/publisher"
	"mem_pan/services/study-service/internal/repository"
	"mem_pan/services/study-service/internal/service"
	"mem_pan/services/study-service/pb"
)

func main() {
	slogger := logger.New("study-service")
	slog.SetDefault(slogger)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pgxCfg, err := pgx.ParseConfig(cfg.DBUrl)
	if err != nil {
		log.Fatal("parse db url:", err)
	}
	pgxCfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	database := stdlib.OpenDB(*pgxCfg)
	defer database.Close()

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(25)
	database.SetConnMaxLifetime(5 * time.Minute)

	if err := database.Ping(); err != nil {
		log.Fatal("db ping:", err)
	}

	authClient, err := authclient.NewGRPCClient(cfg.AuthServiceAddress)
	if err != nil {
		log.Fatal("auth client:", err)
	}
	defer authClient.Close()

	deckClient, err := deckclient.NewGRPCClient(cfg.DeckServiceAddress)
	if err != nil {
		log.Fatal("deck client:", err)
	}
	defer deckClient.Close()

	billingClient, err := billingclient.NewGRPCClient(cfg.BillingServiceAddress)
	if err != nil {
		log.Fatal("billing client:", err)
	}
	defer billingClient.Close()

	userCardRepo := repository.NewUserCardRepository(database)
	sessionRepo := repository.NewStudySessionRepository(database)
	sessionCardRepo := repository.NewSessionCardRepository(database)
	revlogRepo := repository.NewRevlogRepository(database)
	weightsRepo := repository.NewFsrsWeightsRepository(database)
	deckSettingsRepo := repository.NewDeckSettingsRepository(database)
	revshareRepo := repository.NewRevshareRepository(database)

	var pub publisher.EventPublisher
	if cfg.PubSubProjectID != "" {
		pub = publisher.NewPubSubPublisher(cfg.PubSubProjectID, cfg.PubSubTopic)
	} else {
		pub = publisher.NewNoopPublisher()
	}

	studySvc := service.NewStudyService(
		userCardRepo,
		sessionRepo,
		sessionCardRepo,
		revlogRepo,
		weightsRepo,
		deckClient,
		billingClient,
		revshareRepo,
		pub,
	)
	settingsSvc := service.NewSettingsService(deckSettingsRepo)

	// FSRS weight-optimization cron. Wired only when moderation-fsrs-service is
	// reachable; otherwise POST /internal/fsrs/optimize replies 503.
	var optimizer *fsrsopt.Optimizer
	if cfg.ModerationServiceAddress != "" {
		moderationClient, err := moderationclient.NewGRPCClient(cfg.ModerationServiceAddress)
		if err != nil {
			log.Fatal("moderation client:", err)
		}
		defer moderationClient.Close()
		optimizer = fsrsopt.New(revlogRepo, weightsRepo, moderationClient,
			cfg.FsrsOptimizeMinReviews, cfg.FsrsOptimizeMaxUsers)
		log.Printf("fsrs optimization cron enabled (min_reviews=%d max_users=%d)",
			cfg.FsrsOptimizeMinReviews, cfg.FsrsOptimizeMaxUsers)
	} else {
		log.Println("MODERATION_SERVICE_ADDRESS unset — fsrs optimization cron disabled")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	gapiServer := gapi.NewServer(studySvc, settingsSvc, authClient)
	grpcServer := buildGRPCServer(gapiServer, slogger)
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPGateway(cfg, gapiServer, grpcServer, optimizer, studySvc, slogger)

	<-quit
	log.Println("study-service shutting down")
}

func buildGRPCServer(server *gapi.Server, logger *slog.Logger) *grpc.Server {
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
	pb.RegisterStudyServiceServer(grpcServer, server)
	reflection.Register(grpcServer)
	return grpcServer
}

func runStandaloneGRPC(grpcServer *grpc.Server, addr string) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	log.Printf("gRPC server listening on %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func runHTTPGateway(cfg config.Config, gapiServer *gapi.Server, grpcServer *grpc.Server, optimizer *fsrsopt.Optimizer, studySvc service.StudyService, logger *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	if err := pb.RegisterStudyServiceHandlerServer(ctx, grpcMux, gapiServer); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	httpMux.HandleFunc("/internal/fsrs/optimize", fsrsOptimizeHandler(cfg, optimizer))
	httpMux.HandleFunc("/internal/revshare/calculate", revshareCalculateHandler(cfg, studySvc))
	httpMux.Handle("/", grpcMux)

	wrapped := middleware.HTTPLogger(logger)(withCORS(httpMux))
	mixed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		wrapped.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:         cfg.HTTPServerAddress,
		Handler:      h2c.NewHandler(mixed, &http2.Server{}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("HTTP+gRPC server listening on %s", cfg.HTTPServerAddress)
	log.Printf("Swagger UI available at http://%s/swagger/", cfg.HTTPServerAddress)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP gateway failed: %v", err)
	}
}

type revshareCalculateRequest struct {
	PoolMonth      string  `json:"pool_month"`
	GrossAmountVND int64   `json:"gross_amount_vnd"`
	PoolRate       float64 `json:"pool_rate"`
	MinLearners    int32   `json:"min_learners"`
	CreatorCapRate float64 `json:"creator_cap_rate"`
}

func revshareCalculateHandler(cfg config.Config, studySvc service.StudyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cfg.CronSecret != "" && r.Header.Get("X-Cron-Secret") != cfg.CronSecret {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req revshareCalculateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		month, err := time.Parse("2006-01", req.PoolMonth)
		if err != nil {
			http.Error(w, "pool_month must be YYYY-MM", http.StatusBadRequest)
			return
		}
		pool, earnings, err := studySvc.CalculateMonthlyRevShare(r.Context(), month, req.GrossAmountVND, req.PoolRate, req.MinLearners, req.CreatorCapRate)
		if err != nil {
			log.Printf("[revshare] calculate failed: %v", err)
			http.Error(w, "revshare calculation failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pool":     pool,
			"earnings": earnings,
		})
	}
}

// fsrsOptimizeHandler triggers one FSRS weight-optimization pass. It is meant to
// be called by Cloud Scheduler once a day, guarded by a shared secret. The batch
// can run for minutes, so it overrides the server's 15s WriteTimeout per request.
func fsrsOptimizeHandler(cfg config.Config, optimizer *fsrsopt.Optimizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cfg.CronSecret != "" && r.Header.Get("X-Cron-Secret") != cfg.CronSecret {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if optimizer == nil {
			http.Error(w, "fsrs optimization not configured", http.StatusServiceUnavailable)
			return
		}

		// Lift the write deadline so a multi-minute batch isn't cut off mid-run.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(25 * time.Minute))

		summary, err := optimizer.RunOnce(r.Context())
		if err != nil {
			log.Printf("[fsrs-opt] run failed: %v", err)
			http.Error(w, "optimization run failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summary)
	}
}
