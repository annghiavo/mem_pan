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

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

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

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("postgres", cfg.DBUrl)
	if err != nil {
		log.Fatal("open db:", err)
	}
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
	go runGRPCServer(cfg, gapiServer)
	go runHTTPGateway(cfg, gapiServer)

	<-quit
	log.Println("auth-service shutting down")
}

func runGRPCServer(cfg config.Config, server *gapi.Server) {
	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, server)
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

func runHTTPGateway(cfg config.Config, gapiServer *gapi.Server) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := pb.RegisterAuthServiceHandlerFromEndpoint(ctx, grpcMux, cfg.GRPCServerAddress, opts); err != nil {
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
