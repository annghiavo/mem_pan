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
	"mem_pan/services/admin-service/config"
	"mem_pan/services/admin-service/doc"
	"mem_pan/services/admin-service/internal/authclient"
	"mem_pan/services/admin-service/internal/deckclient"
	"mem_pan/services/admin-service/internal/gapi"
	"mem_pan/services/admin-service/internal/notifyclient"
	"mem_pan/services/admin-service/internal/publisher"
	"mem_pan/services/admin-service/internal/repository"
	"mem_pan/services/admin-service/internal/service"
	"mem_pan/services/admin-service/internal/subscriber"
	pb "mem_pan/services/admin-service/pb/proto"
)

func main() {
	slogger := logger.New("admin-service")
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

	notifyClient, err := notifyclient.NewGRPCClient(cfg.NotificationServiceAddress)
	if err != nil {
		log.Fatal("notification client:", err)
	}
	defer notifyClient.Close()

	deckClient, err := deckclient.NewGRPCClient(cfg.DeckServiceAddress)
	if err != nil {
		log.Fatal("deck client:", err)
	}
	defer deckClient.Close()

	var pub publisher.EventPublisher
	if cfg.PubSubProjectID != "" {
		pub = publisher.NewPubSubPublisher(cfg.PubSubProjectID, cfg.PubSubTopic)
	} else {
		pub = publisher.NewNoopPublisher()
	}

	reportRepo := repository.NewReportRepository(database)
	reportSvc := service.NewReportService(reportRepo, authClient, deckClient, pub)

	server := gapi.NewServer(reportSvc, reportRepo, authClient, notifyClient, deckClient)

	subHandler := subscriber.NewHandler(reportRepo)
	pushHandler := subscriber.NewPushHandler(subHandler, cfg.PubSubPushSecret)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	grpcServer := buildGRPCServer(server, slogger)
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPGateway(cfg, server, pushHandler, grpcServer, slogger)

	<-quit
	log.Println("admin-service shutting down")
}

func buildGRPCServer(server *gapi.Server, logger *slog.Logger) *grpc.Server {
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
	pb.RegisterAdminServiceServer(grpcServer, server)
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

func runHTTPGateway(cfg config.Config, gapiServer *gapi.Server, pushHandler *subscriber.PushHandler, grpcServer *grpc.Server, logger *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	if err := pb.RegisterAdminServiceHandlerServer(ctx, grpcMux, gapiServer); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	httpMux.Handle("/internal/pubsub", pushHandler)
	httpMux.Handle("/", grpcMux)

	wrapped := middleware.HTTPLogger(logger)(withCORS(httpMux))
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
	log.Printf("Swagger UI available at http://%s/swagger/", cfg.HTTPServerAddress)
	log.Printf("Pub/Sub push endpoint: POST %s/internal/pubsub", cfg.HTTPServerAddress)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP gateway failed: %v", err)
	}
}
