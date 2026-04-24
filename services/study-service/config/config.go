package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBUrl               string
	GRPCServerAddress   string
	HTTPServerAddress   string
	AuthServiceAddress  string
	DeckServiceAddress  string
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:              getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress:  getEnv("GRPC_SERVER_ADDRESS", ":9092"),
		HTTPServerAddress:  getEnv("HTTP_SERVER_ADDRESS", ":8082"),
		AuthServiceAddress: getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),
		DeckServiceAddress: getEnv("DECK_SERVICE_ADDRESS", "localhost:9091"),
	}
	if cfg.DBUrl == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
