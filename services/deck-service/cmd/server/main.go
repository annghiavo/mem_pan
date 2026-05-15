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

	"mem_pan/services/deck-service/config"
	"mem_pan/services/deck-service/doc"
	"mem_pan/services/deck-service/internal/authclient"
	"mem_pan/services/deck-service/internal/gapi"
	"mem_pan/services/deck-service/internal/publisher"
	"mem_pan/services/deck-service/internal/repository"
	"mem_pan/services/deck-service/internal/service"
	"mem_pan/services/deck-service/internal/uploader"
	"mem_pan/services/deck-service/pb"
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

	folderRepo := repository.NewFolderRepository(database)
	deckRepo := repository.NewDeckRepository(database)
	noteRepo := repository.NewNoteRepository(database)
	cardRepo := repository.NewCardRepository(database)
	folderDeckRepo := repository.NewFolderDeckRepository(database)

	var pub publisher.EventPublisher
	if cfg.PubSubProjectID != "" {
		pub = publisher.NewPubSubPublisher(cfg.PubSubProjectID, cfg.PubSubTopic)
	} else {
		pub = publisher.NewNoopPublisher()
	}

	folderSvc := service.NewFolderService(folderRepo, folderDeckRepo, deckRepo, pub)
	deckSvc := service.NewDeckService(deckRepo, cardRepo, pub)
	cardSvc := service.NewCardService(cardRepo, noteRepo, deckRepo, pub)

	var imageUploader uploader.ImageUploader
	if cfg.CloudinaryURL != "" {
		imageUploader, err = uploader.NewCloudinary(cfg.CloudinaryURL)
		if err != nil {
			log.Fatal("cloudinary init:", err)
		}
	} else {
		log.Println("CLOUDINARY_URL not set — image upload disabled")
	}

	server := gapi.NewServer(folderSvc, deckSvc, cardSvc, authClient, imageUploader, pub)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go runGRPCServer(cfg, server)
	go runHTTPGateway(cfg, server)

	<-quit
	log.Println("deck-service shutting down")
}

func runGRPCServer(cfg config.Config, server *gapi.Server) {
	grpcServer := grpc.NewServer()
	pb.RegisterDeckServiceServer(grpcServer, server)
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

func runHTTPGateway(cfg config.Config, srv *gapi.Server) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := pb.RegisterDeckServiceHandlerFromEndpoint(ctx, grpcMux, cfg.GRPCServerAddress, opts); err != nil {
		log.Fatalf("failed to register HTTP gateway: %v", err)
	}

	swaggerFiles, err := fs.Sub(doc.SwaggerFS, "swagger")
	if err != nil {
		log.Fatalf("swagger fs.Sub: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles))))
	httpMux.HandleFunc("POST /v1/import/parse", srv.ServeParseImportFile)
	httpMux.HandleFunc("POST /v1/cards/upload-image", srv.ServeUploadCardImage)
	httpMux.HandleFunc("POST /v1/decks/{deck_id}/cards", srv.ServeCreateCard)
	httpMux.HandleFunc("PUT /v1/cards/{card_id}", srv.ServeUpdateCard)
	httpMux.Handle("/", grpcMux)

	httpServer := &http.Server{
		Addr:         cfg.HTTPServerAddress,
		Handler:      withCORS(httpMux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("HTTP gateway listening on %s", cfg.HTTPServerAddress)
	log.Printf("Swagger UI available at http://%s/swagger/", cfg.HTTPServerAddress)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP gateway failed: %v", err)
	}
}
