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

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"mem_pan/pkg/logger"
	"mem_pan/pkg/middleware"
	"mem_pan/services/auth-service/config"
	"mem_pan/services/auth-service/doc"
	"mem_pan/services/auth-service/internal/gapi"
	"mem_pan/services/auth-service/internal/publisher"
	"mem_pan/services/auth-service/internal/repository"
	"mem_pan/services/auth-service/internal/service"
	"mem_pan/services/auth-service/internal/token"
	"mem_pan/services/auth-service/pb"
)

func main() {
	_ = godotenv.Load("app.env")

	slogger := logger.New("auth-service")
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
	db := stdlib.OpenDB(*pgxCfg)
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("db ping:", err)
	}

	tokenMaker, err := token.NewPasetoMaker(cfg.PasetoSymmetricKey)
	if err != nil {
		log.Fatal("token maker:", err)
	}

	cld, err := cloudinary.NewFromURL(cfg.CloudinaryURL)
	if err != nil {
		log.Fatal("cloudinary:", err)
	}

	userRepo := repository.NewUserRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	verifyTokenRepo := repository.NewVerificationTokenRepository(db)
	var pub publisher.EventPublisher
	if cfg.PubSubProjectID != "" {
		pub = publisher.NewPubSubPublisher(cfg.PubSubProjectID, cfg.PubSubTopic)
	} else {
		pub = publisher.NewNoopPublisher()
	}

	authSvc := service.NewAuthService(
		userRepo, refreshTokenRepo, verifyTokenRepo,
		tokenMaker, pub,
		cfg.AccessTokenDuration, cfg.RefreshTokenDuration,
		cfg.VerificationTokenDuration, cfg.ResetTokenDuration,
	)
	userSvc := service.NewUserService(userRepo, pub)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	gapiServer := gapi.NewServer(authSvc, userSvc, tokenMaker, cld, pub)
	grpcServer := buildGRPCServer(gapiServer, slogger)
	// Local docker-compose still needs a dedicated gRPC port for service-to-service
	// calls on :9090. Cloud Run only exposes a single port, so service-to-service
	// calls land on the merged HTTP server below.
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPGateway(cfg, gapiServer, grpcServer, slogger)

	<-quit
	log.Println("auth-service shutting down")
}

func buildGRPCServer(server *gapi.Server, logger *slog.Logger) *grpc.Server {
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
	pb.RegisterAuthServiceServer(grpcServer, server)
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

func runHTTPGateway(cfg config.Config, gapiServer *gapi.Server, grpcServer *grpc.Server, logger *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	// In-process handler registration: grpc-gateway invokes the gRPC service
	// directly without going through a TCP dial. Avoids the localhost loop and
	// works in single-port environments (Cloud Run).
	if err := pb.RegisterAuthServiceHandlerServer(ctx, grpcMux, gapiServer); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	httpMux.HandleFunc("/v1/users/me/avatar", gapiServer.UploadAvatarHTTP)
	httpMux.Handle("/", grpcMux)

	// Mux gRPC frames (HTTP/2 with Content-Type: application/grpc) into the
	// gRPC server; everything else goes through the REST/HTTP gateway. h2c lets
	// Cloud Run's HTTP/2 cleartext traffic from the load balancer reach us.
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
