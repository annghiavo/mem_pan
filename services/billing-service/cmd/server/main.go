package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"mem_pan/pkg/logger"
	"mem_pan/pkg/middleware"
	"mem_pan/services/billing-service/config"
	"mem_pan/services/billing-service/internal/authclient"
	"mem_pan/services/billing-service/internal/gapi"
	"mem_pan/services/billing-service/internal/httpapi"
	"mem_pan/services/billing-service/internal/payos"
	"mem_pan/services/billing-service/internal/repository"
	"mem_pan/services/billing-service/internal/service"
	pb "mem_pan/services/billing-service/pb"
)

func main() {
	_ = godotenv.Load("app.env")

	slogger := logger.New("billing-service")
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
	payosClient := payos.NewClient(payos.Config{
		BaseURL:           cfg.PayOSBaseURL,
		ClientID:          cfg.PayOSClientID,
		APIKey:            cfg.PayOSAPIKey,
		ChecksumKey:       cfg.PayOSChecksumKey,
		PayoutClientID:    cfg.PayOSPayoutClientID,
		PayoutAPIKey:      cfg.PayOSPayoutAPIKey,
		PayoutChecksumKey: cfg.PayOSPayoutChecksumKey,
	})
	billingSvc := service.New(repo, payosClient, authClient, cfg.PlusMonthlyAmountVND, cfg.PlusYearlyAmountVND, cfg.DefaultReturnURL, cfg.DefaultCancelURL)

	gapiServer := gapi.NewServer(billingSvc)
	httpHandler := httpapi.NewHandler(billingSvc, authClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	grpcServer := buildGRPCServer(gapiServer, slogger)
	if cfg.GRPCServerAddress != "" && cfg.GRPCServerAddress != cfg.HTTPServerAddress {
		go runStandaloneGRPC(grpcServer, cfg.GRPCServerAddress)
	}
	go runHTTPServer(cfg, httpHandler, grpcServer, slogger)

	<-quit
	log.Println("billing-service shutting down")
}

func buildGRPCServer(server *gapi.Server, logger *slog.Logger) *grpc.Server {
	s := grpc.NewServer(grpc.UnaryInterceptor(middleware.UnaryServerLogger(logger)))
	pb.RegisterBillingServiceServer(s, server)
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

func runHTTPServer(cfg config.Config, api *httpapi.Handler, grpcServer *grpc.Server, logger *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx

	mux := http.NewServeMux()
	api.Register(mux)

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
	log.Printf("Billing API: POST %s/v1/billing/checkout", cfg.HTTPServerAddress)
	log.Printf("Billing API: POST %s/v1/billing/confirm", cfg.HTTPServerAddress)
	log.Printf("PayOS webhook: POST %s/v1/billing/webhooks/payos", cfg.HTTPServerAddress)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
