package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DBUrl              string
	GRPCServerAddress  string
	HTTPServerAddress  string
	AuthServiceAddress string
	DeckServiceAddress string
	PubSubProjectID    string
	PubSubTopic        string

	// FSRS weight-optimization cron (POST /internal/fsrs/optimize).
	ModerationServiceAddress string // gRPC addr of moderation-fsrs-service; empty disables the cron
	FsrsOptimizeMinReviews   int64  // min reviews a user needs before re-tuning
	FsrsOptimizeMaxUsers     int    // safety cap on users per run (0 = no cap)
	CronSecret               string // shared secret required in X-Cron-Secret header
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:              getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress:  getEnv("GRPC_SERVER_ADDRESS", ":9092"),
		HTTPServerAddress:  getEnv("HTTP_SERVER_ADDRESS", ":8082"),
		AuthServiceAddress: getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),
		DeckServiceAddress: getEnv("DECK_SERVICE_ADDRESS", "localhost:9091"),
		PubSubProjectID:    getEnv("PUBSUB_PROJECT_ID", ""),
		PubSubTopic:        getEnv("PUBSUB_TOPIC", "study-events"),

		ModerationServiceAddress: getEnv("MODERATION_SERVICE_ADDRESS", ""),
		// 1000 is the FSRS-recommended minimum for trustworthy optimization;
		// below ~400 the fit overfits and can underperform the defaults.
		// Lower it (e.g. 50) for a demo so the cron actually has work to do.
		FsrsOptimizeMinReviews: getEnvInt64("FSRS_OPTIMIZE_MIN_REVIEWS", 10),
		FsrsOptimizeMaxUsers:   int(getEnvInt64("FSRS_OPTIMIZE_MAX_USERS", 200)),
		CronSecret:             getEnv("CRON_SECRET", ""),
	}
	if cfg.DBUrl == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return defaultVal
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
