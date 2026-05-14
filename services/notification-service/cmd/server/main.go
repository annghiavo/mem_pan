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

	"mem_pan/services/notification-service/config"
	"mem_pan/services/notification-service/doc"
	"mem_pan/services/notification-service/internal/authclient"
	"mem_pan/services/notification-service/internal/fcm"
	"mem_pan/services/notification-service/internal/gapi"
	"mem_pan/services/notification-service/internal/mailer"
	"mem_pan/services/notification-service/internal/repository"
	"mem_pan/services/notification-service/internal/service"
	"mem_pan/services/notification-service/internal/subscriber"
	pb "mem_pan/services/notification-service/pb"
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

	// Mailer — use noop if SMTP is not configured.
	var m mailer.Mailer
	if cfg.SMTPHost != "" {
		m = mailer.New(mailer.Config{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.EmailFrom,
		})
	} else {
		log.Println("SMTP_HOST not set — email notifications disabled")
		m = mailer.NewNoop()
	}

	// FCM sender — use noop if not configured.
	var fcmSender fcm.Sender
	if cfg.FCMProjectID != "" {
		fcmSender, err = fcm.New(context.Background(), cfg.FCMProjectID, cfg.FCMCredentialsFile)
		if err != nil {
			log.Fatal("fcm init:", err)
		}
	} else {
		log.Println("FCM_PROJECT_ID not set — push notifications disabled")
		fcmSender = fcm.NewNoop()
	}

	repo := repository.New(database)
	svc := service.New(repo, m, fcmSender, service.Config{AppBaseURL: cfg.AppBaseURL})

	handler := subscriber.NewHandler(svc)
	pushHandler := subscriber.NewPushHandler(handler, cfg.PubSubPushSecret)

	grpcServer := gapi.NewServer(svc, authClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go runGRPCServer(cfg, grpcServer)
	go runHTTPServer(cfg, grpcServer, pushHandler)

	<-quit
	log.Println("notification-service shutting down")
}

func runGRPCServer(cfg config.Config, server *gapi.Server) {
	s := grpc.NewServer()
	pb.RegisterNotificationServiceServer(s, server)
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

func runHTTPServer(cfg config.Config, grpcServer *gapi.Server, pushHandler *subscriber.PushHandler) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := pb.RegisterNotificationServiceHandlerFromEndpoint(ctx, grpcMux, cfg.GRPCServerAddress, opts); err != nil {
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
