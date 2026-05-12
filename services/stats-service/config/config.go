package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBUrl              string
	GRPCServerAddress  string
	HTTPServerAddress  string
	AuthServiceAddress string

	PubSubProjectID string
	UserEventsSub   string
	DeckEventsSub   string
	StudyEventsSub  string
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:             getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress: getEnv("GRPC_SERVER_ADDRESS", ":9094"),
		HTTPServerAddress: getEnv("HTTP_SERVER_ADDRESS", ":8084"),

		AuthServiceAddress: getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),

		PubSubProjectID: getEnv("PUBSUB_PROJECT_ID", ""),
		UserEventsSub:   getEnv("PUBSUB_USER_EVENTS_SUB", "stats-user-events-sub"),
		DeckEventsSub:   getEnv("PUBSUB_DECK_EVENTS_SUB", "stats-deck-events-sub"),
		StudyEventsSub:  getEnv("PUBSUB_STUDY_EVENTS_SUB", "stats-study-events-sub"),
	}
	if cfg.DBUrl == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.PubSubProjectID == "" {
		return Config{}, fmt.Errorf("PUBSUB_PROJECT_ID is required")
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
