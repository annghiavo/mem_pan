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
	slogger := logger.New("deck-service")
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

	grpcServer := buildGRPCServer(server, slogger)
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPGateway(cfg, server, grpcServer, slogger)

	<-quit
	log.Println("deck-service shutting down")
}

func buildGRPCServer(server *gapi.Server, logger *slog.Logger) *grpc.Server {
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
	pb.RegisterDeckServiceServer(grpcServer, server)
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

func dispatchMultipart(multipartHandler http.HandlerFunc, fallback http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			multipartHandler(w, r)
			return
		}
		fallback.ServeHTTP(w, r)
	}
}

func runHTTPGateway(cfg config.Config, srv *gapi.Server, grpcServer *grpc.Server, logger *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	if err := pb.RegisterDeckServiceHandlerServer(ctx, grpcMux, srv); err != nil {
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
	// Card create/update accept either multipart (with an image file) or JSON
	// (text-only / image_url). Dispatch by Content-Type so JSON requests fall
	// through to the grpc-gateway instead of hitting the multipart handler,
	// which would reject them with "invalid multipart form".
	httpMux.HandleFunc("POST /v1/decks/{deck_id}/cards", dispatchMultipart(srv.ServeCreateCard, grpcMux))
	httpMux.HandleFunc("PUT /v1/cards/{card_id}", dispatchMultipart(srv.ServeUpdateCard, grpcMux))
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
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP gateway failed: %v", err)
	}
}
