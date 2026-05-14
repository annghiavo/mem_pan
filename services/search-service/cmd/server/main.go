package main

import (
	"context"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"mem_pan/services/search-service/config"
	"mem_pan/services/search-service/doc"
	"mem_pan/services/search-service/internal/authclient"
	"mem_pan/services/search-service/internal/es"
	"mem_pan/services/search-service/internal/gapi"
	"mem_pan/services/search-service/internal/service"
	"mem_pan/services/search-service/internal/subscriber"
	pb "mem_pan/services/search-service/pb"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	esClient, err := es.New(cfg.ElasticsearchURLs, cfg.ElasticsearchAPIKey, es.Indices{
		Deck:   cfg.DeckIndex,
		Folder: cfg.FolderIndex,
		Card:   cfg.CardIndex,
		User:   cfg.UserIndex,
	})
	if err != nil {
		log.Fatal("elasticsearch:", err)
	}
	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := esClient.EnsureIndices(bootstrapCtx); err != nil {
		bootstrapCancel()
		log.Fatal("ensure indices:", err)
	}
	bootstrapCancel()

	authClient, err := authclient.NewGRPCClient(cfg.AuthServiceAddress)
	if err != nil {
		log.Fatal("auth client:", err)
	}
	defer authClient.Close()

	svc := service.New(esClient)
	handler := subscriber.NewHandler(svc)
	pushHandler := subscriber.NewPushHandler(handler, cfg.PubSubPushSecret)

	grpcServer := gapi.NewServer(svc, authClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go runGRPCServer(cfg, grpcServer)
	go runHTTPServer(cfg, pushHandler)

	<-quit
	log.Println("search-service shutting down")
}

func runGRPCServer(cfg config.Config, server *gapi.Server) {
	s := grpc.NewServer()
	pb.RegisterSearchServiceServer(s, server)
	reflection.Register(s)

	lis, err := net.Listen("tcp", cfg.GRPCServerAddress)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.GRPCServerAddress, err)
	}

	log.Printf("gRPC server listening on %s", cfg.GRPCServerAddress)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func runHTTPServer(cfg config.Config, pushHandler *subscriber.PushHandler) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := pb.RegisterSearchServiceHandlerFromEndpoint(ctx, grpcMux, cfg.GRPCServerAddress, opts); err != nil {
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

	httpServer := &http.Server{
		Addr:         cfg.HTTPServerAddress,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("HTTP server listening on %s", cfg.HTTPServerAddress)
	log.Printf("Pub/Sub push endpoint: POST %s/internal/pubsub", cfg.HTTPServerAddress)
	log.Printf("Swagger UI: http://%s/swagger/", cfg.HTTPServerAddress)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
