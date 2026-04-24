package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	DBUrl                     string
	GRPCServerAddress         string
	HTTPServerAddress         string
	PasetoSymmetricKey        string
	AccessTokenDuration       time.Duration
	RefreshTokenDuration      time.Duration
	VerificationTokenDuration time.Duration
	ResetTokenDuration        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:                     getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress:         getEnv("GRPC_SERVER_ADDRESS", ":9090"),
		HTTPServerAddress:         getEnv("HTTP_SERVER_ADDRESS", ":8080"),
		PasetoSymmetricKey:        getEnv("PASETO_SYMMETRIC_KEY", ""),
		AccessTokenDuration:       getDuration("ACCESS_TOKEN_DURATION", 15*time.Minute),
		RefreshTokenDuration:      getDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		VerificationTokenDuration: getDuration("VERIFICATION_TOKEN_DURATION", 24*time.Hour),
		ResetTokenDuration:        getDuration("RESET_TOKEN_DURATION", 1*time.Hour),
	}
	if cfg.DBUrl == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.PasetoSymmetricKey) != 32 {
		return Config{}, fmt.Errorf("PASETO_SYMMETRIC_KEY must be exactly 32 characters, got %d", len(cfg.PasetoSymmetricKey))
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getDuration(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultVal
	}
	return d
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
