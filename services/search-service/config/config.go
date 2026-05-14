package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	GRPCServerAddress  string
	HTTPServerAddress  string
	AuthServiceAddress string

	PubSubPushSecret string

	// Elasticsearch
	ElasticsearchURLs   []string
	ElasticsearchAPIKey string

	// Index names — overridable so different envs can share a cluster.
	DeckIndex   string
	FolderIndex string
	CardIndex   string
	UserIndex   string
}

func Load() (Config, error) {
	cfg := Config{
		GRPCServerAddress:  getEnv("GRPC_SERVER_ADDRESS", ":9096"),
		HTTPServerAddress:  getEnv("HTTP_SERVER_ADDRESS", ":8086"),
		AuthServiceAddress: getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),

		PubSubPushSecret: getEnv("PUBSUB_PUSH_SECRET", ""),

		ElasticsearchURLs:   splitCSV(getEnv("ELASTICSEARCH_URL", "")),
		ElasticsearchAPIKey: getEnv("ELASTICSEARCH_API_KEY", ""),

		DeckIndex:   getEnv("ES_DECK_INDEX", "mempan-decks"),
		FolderIndex: getEnv("ES_FOLDER_INDEX", "mempan-folders"),
		CardIndex:   getEnv("ES_CARD_INDEX", "mempan-cards"),
		UserIndex:   getEnv("ES_USER_INDEX", "mempan-users"),
	}

	if len(cfg.ElasticsearchURLs) == 0 {
		return Config{}, fmt.Errorf("ELASTICSEARCH_URL is required")
	}
	if cfg.ElasticsearchAPIKey == "" {
		return Config{}, fmt.Errorf("ELASTICSEARCH_API_KEY is required")
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

