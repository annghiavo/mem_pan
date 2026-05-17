package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBUrl                      string
	GRPCServerAddress          string
	HTTPServerAddress          string
	AuthServiceAddress         string
	NotificationServiceAddress string
	DeckServiceAddress         string
	PubSubPushSecret           string
	PubSubProjectID            string
	PubSubTopic                string
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:                      getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress:          getEnv("GRPC_SERVER_ADDRESS", ":9093"),
		HTTPServerAddress:          getEnv("HTTP_SERVER_ADDRESS", ":8083"),
		AuthServiceAddress:         getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),
		NotificationServiceAddress: getEnv("NOTIFICATION_SERVICE_ADDRESS", "localhost:9095"),
		DeckServiceAddress:         getEnv("DECK_SERVICE_ADDRESS", "localhost:9091"),
		PubSubPushSecret:           getEnv("PUBSUB_PUSH_SECRET", ""),
		PubSubProjectID:            getEnv("PUBSUB_PROJECT_ID", ""),
		PubSubTopic:                getEnv("PUBSUB_TOPIC", "user-events"),
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
