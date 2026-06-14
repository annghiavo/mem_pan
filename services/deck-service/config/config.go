package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBUrl                 string
	GRPCServerAddress     string
	HTTPServerAddress     string
	AuthServiceAddress    string
	StudyServiceAddress   string
	BillingServiceAddress string
	CloudinaryURL         string
	PubSubProjectID       string
	PubSubTopic           string
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:                 getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress:     getEnv("GRPC_SERVER_ADDRESS", ":9091"),
		HTTPServerAddress:     getEnv("HTTP_SERVER_ADDRESS", ":8081"),
		AuthServiceAddress:    getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),
		BillingServiceAddress: getEnv("BILLING_SERVICE_ADDRESS", "localhost:9098"),
		// Optional: enables the learner count on the deck detail page. When
		// empty the count is simply omitted (left at 0).
		StudyServiceAddress: os.Getenv("STUDY_SERVICE_ADDRESS"),
		CloudinaryURL:       os.Getenv("CLOUDINARY_URL"),
		PubSubProjectID:     getEnv("PUBSUB_PROJECT_ID", ""),
		PubSubTopic:         getEnv("PUBSUB_TOPIC", "deck-events"),
	}
	if cfg.DBUrl == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AuthServiceAddress == "" {
		return Config{}, fmt.Errorf("AUTH_SERVICE_ADDRESS is required")
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
