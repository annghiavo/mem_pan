package main

import (
	"context"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
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

	gapiServer := gapi.NewServer(svc, authClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	grpcServer := buildGRPCServer(gapiServer)
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPServer(cfg, gapiServer, pushHandler, grpcServer)

	<-quit
	log.Println("search-service shutting down")
}

func buildGRPCServer(server *gapi.Server) *grpc.Server {
	s := grpc.NewServer()
	pb.RegisterSearchServiceServer(s, server)
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

func runHTTPServer(cfg config.Config, gapiServer *gapi.Server, pushHandler *subscriber.PushHandler, grpcServer *grpc.Server) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	if err := pb.RegisterSearchServiceHandlerServer(ctx, grpcMux, gapiServer); err != nil {
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

	mixed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		withCORS(mux).ServeHTTP(w, r)
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
