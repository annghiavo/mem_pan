package main

import (
	"context"
	"database/sql"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

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
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	database, err := sql.Open("postgres", cfg.DBUrl)
	if err != nil {
		log.Fatal("open db:", err)
	}
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

	psClient, err := pubsub.NewClient(context.Background(), cfg.PubSubProjectID)
	if err != nil {
		log.Fatal("pubsub client:", err)
	}
	defer psClient.Close()

	handler := subscriber.NewHandler(repo)
	sub := subscriber.New(psClient, handler)

	server := gapi.NewServer(statsSvc, authClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-quit
		log.Println("stats-service shutting down")
		cancel()
	}()

	go runGRPCServer(cfg, server)
	go runHTTPGateway(cfg)

	// Subscriber blocks until ctx is cancelled
	sub.Start(ctx, cfg.UserEventsSub, cfg.DeckEventsSub, cfg.StudyEventsSub)
}

func runGRPCServer(cfg config.Config, server *gapi.Server) {
	grpcServer := grpc.NewServer()
	pb.RegisterStatsServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", cfg.GRPCServerAddress)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.GRPCServerAddress, err)
	}

	log.Printf("gRPC server listening on %s", cfg.GRPCServerAddress)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func runHTTPGateway(cfg config.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := pb.RegisterStatsServiceHandlerFromEndpoint(ctx, grpcMux, cfg.GRPCServerAddress, opts); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	httpMux.Handle("/", grpcMux)

	httpServer := &http.Server{
		Addr:         cfg.HTTPServerAddress,
		Handler:      httpMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("HTTP gateway listening on %s", cfg.HTTPServerAddress)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP gateway failed: %v", err)
	}
}
