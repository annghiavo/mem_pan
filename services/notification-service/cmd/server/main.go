package main

import (
	"context"
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
	"mem_pan/services/notification-service/config"
	"mem_pan/services/notification-service/doc"
	"mem_pan/services/notification-service/internal/authclient"
	"mem_pan/services/notification-service/internal/fcm"
	"mem_pan/services/notification-service/internal/gapi"
	"mem_pan/services/notification-service/internal/mailer"
	"mem_pan/services/notification-service/internal/repository"
	"mem_pan/services/notification-service/internal/scheduler"
	"mem_pan/services/notification-service/internal/service"
	"mem_pan/services/notification-service/internal/statsclient"
	"mem_pan/services/notification-service/internal/studyclient"
	"mem_pan/services/notification-service/internal/subscriber"
	pb "mem_pan/services/notification-service/pb"
)

func main() {
	slogger := logger.New("notification-service")
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

	repo := repository.New(database)
	templateCache := mailer.NewCachedStore(mailer.NewRepoStore(repo))

	// Mailer — use noop if SMTP is not configured.
	var m mailer.Mailer
	if cfg.SMTPHost != "" {
		m = mailer.New(mailer.Config{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.EmailFrom,
		}, templateCache)
	} else {
		log.Println("SMTP_HOST not set — email notifications disabled")
		m = mailer.NewNoop()
	}

	// FCM sender — use noop if not configured.
	var fcmSender fcm.Sender
	if cfg.FCMProjectID != "" {
		fcmSender, err = fcm.New(context.Background(), cfg.FCMProjectID, cfg.FCMCredentialsFile)
		if err != nil {
			log.Fatal("fcm init:", err)
		}
	} else {
		log.Println("FCM_PROJECT_ID not set — push notifications disabled")
		fcmSender = fcm.NewNoop()
	}

	svc := service.New(repo, m, fcmSender, templateCache, service.Config{AppBaseURL: cfg.AppBaseURL})

	// Optional internal clients for reminder cron handlers. If either address
	// is missing the scheduler stays nil and the two cron events log + skip.
	var sched *scheduler.Scheduler
	if cfg.StatsServiceAddress != "" && cfg.StudyServiceAddress != "" {
		statsCli, err := statsclient.NewGRPCClient(cfg.StatsServiceAddress)
		if err != nil {
			log.Fatal("stats client:", err)
		}
		defer statsCli.Close()
		studyCli, err := studyclient.NewGRPCClient(cfg.StudyServiceAddress)
		if err != nil {
			log.Fatal("study client:", err)
		}
		defer studyCli.Close()
		sched = scheduler.New(repo, fcmSender, authClient, statsCli, studyCli)
		log.Println("reminder scheduler wired (stats + study clients ready)")
	} else {
		log.Println("STATS_SERVICE_ADDRESS or STUDY_SERVICE_ADDRESS not set — reminder crons disabled")
	}

	handler := subscriber.NewHandler(svc, authClient, sched)
	pushHandler := subscriber.NewPushHandler(handler, cfg.PubSubPushSecret)

	gapiServer := gapi.NewServer(svc, authClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	grpcServer := buildGRPCServer(gapiServer, slogger)
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPServer(cfg, gapiServer, pushHandler, grpcServer, slogger)

	<-quit
	log.Println("notification-service shutting down")
}

func buildGRPCServer(server *gapi.Server, logger *slog.Logger) *grpc.Server {
	s := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
	pb.RegisterNotificationServiceServer(s, server)
	reflection.Register(s)
	return s
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

func runHTTPServer(cfg config.Config, gapiServer *gapi.Server, pushHandler *subscriber.PushHandler, grpcServer *grpc.Server, logger *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	if err := pb.RegisterNotificationServiceHandlerServer(ctx, grpcMux, gapiServer); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	mux.Handle("/internal/pubsub", pushHandler)
	mux.Handle("/", grpcMux)

	wrapped := middleware.HTTPLogger(logger)(withCORS(mux))
	mixed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		wrapped.ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:         cfg.HTTPServerAddress,
		Handler:      h2c.NewHandler(mixed, &http2.Server{}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("HTTP+gRPC server listening on %s", cfg.HTTPServerAddress)
	log.Printf("Pub/Sub push endpoint: POST %s/internal/pubsub", cfg.HTTPServerAddress)
	log.Printf("Swagger UI: http://%s/swagger/", cfg.HTTPServerAddress)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
