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
	"mem_pan/services/stats-service/config"
	"mem_pan/services/stats-service/doc"
	"mem_pan/services/stats-service/internal/authclient"
	"mem_pan/services/stats-service/internal/gapi"
	"mem_pan/services/stats-service/internal/repository"
	"mem_pan/services/stats-service/internal/service"
	"mem_pan/services/stats-service/internal/subscriber"
	pb "mem_pan/services/stats-service/pb"
)

func main() {
	slogger := logger.New("stats-service")
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
	statsSvc := service.New(repo)

	handler := subscriber.NewHandler(repo)
	pushHandler := subscriber.NewPushHandler(handler, cfg.PubSubPushSecret)

	gapiServer := gapi.NewServer(statsSvc, authClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	grpcServer := buildGRPCServer(gapiServer, slogger)
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPServer(cfg, gapiServer, pushHandler, grpcServer, slogger)

	<-quit
	log.Println("stats-service shutting down")
}

func buildGRPCServer(server *gapi.Server, logger *slog.Logger) *grpc.Server {
	s := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
	pb.RegisterStatsServiceServer(s, server)
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
	if err := pb.RegisterStatsServiceHandlerServer(ctx, grpcMux, gapiServer); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	// Pub/Sub push endpoint — registered before "/" so it takes priority.
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
