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

	// PubSubPushSecret is an optional shared secret appended to the push
	// endpoint URL as ?token=<secret>. Pub/Sub includes it on every request
	// so the handler can reject traffic from other sources.
	PubSubPushSecret string
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:             getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress: getEnv("GRPC_SERVER_ADDRESS", ":9094"),
		HTTPServerAddress: getEnv("HTTP_SERVER_ADDRESS", ":8084"),

		AuthServiceAddress: getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),

		PubSubPushSecret: getEnv("PUBSUB_PUSH_SECRET", ""),
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
