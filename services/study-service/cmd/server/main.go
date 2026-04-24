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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"mem_pan/services/study-service/config"
	"mem_pan/services/study-service/doc"
	"mem_pan/services/study-service/internal/authclient"
	"mem_pan/services/study-service/internal/deckclient"
	"mem_pan/services/study-service/internal/gapi"
	"mem_pan/services/study-service/internal/repository"
	"mem_pan/services/study-service/internal/service"
	"mem_pan/services/study-service/pb"
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

	deckClient, err := deckclient.NewGRPCClient(cfg.DeckServiceAddress)
	if err != nil {
		log.Fatal("deck client:", err)
	}
	defer deckClient.Close()

	userCardRepo := repository.NewUserCardRepository(database)
	sessionRepo := repository.NewStudySessionRepository(database)
	sessionCardRepo := repository.NewSessionCardRepository(database)
	revlogRepo := repository.NewRevlogRepository(database)
	weightsRepo := repository.NewFsrsWeightsRepository(database)

	studySvc := service.NewStudyService(
		userCardRepo,
		sessionRepo,
		sessionCardRepo,
		revlogRepo,
		weightsRepo,
		deckClient,
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go runGRPCServer(cfg, studySvc, authClient)
	go runHTTPGateway(cfg)

	<-quit
	log.Println("study-service shutting down")
}

func runGRPCServer(cfg config.Config, studySvc service.StudyService, authClient authclient.Client) {
	server := gapi.NewServer(studySvc, authClient)

	grpcServer := grpc.NewServer()
	pb.RegisterStudyServiceServer(grpcServer, server)
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

	if err := pb.RegisterStudyServiceHandlerFromEndpoint(ctx, grpcMux, cfg.GRPCServerAddress, opts); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	httpMux.Handle("/", grpcMux)

	srv := &http.Server{
		Addr:         cfg.HTTPServerAddress,
		Handler:      httpMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("HTTP gateway listening on %s", cfg.HTTPServerAddress)
	log.Printf("Swagger UI available at http://%s/swagger/", cfg.HTTPServerAddress)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP gateway failed: %v", err)
	}
}
