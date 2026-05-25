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

	// Optional. Enable the reminder cron handlers
	// (cron.study_reminder and cron.streak_warning).
	StatsServiceAddress string
	StudyServiceAddress string

	PubSubPushSecret string

	// Email (SMTP)
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	EmailFrom    string

	// Firebase Cloud Messaging
	FCMProjectID       string
	FCMCredentialsFile string // path to service account JSON; empty = Application Default Credentials

	// App base URL used to build links in emails
	AppBaseURL string

	// Admin web base URL — used to build the appeal CTA link in the
	// deck-deletion email (deck owners visit this URL to file an appeal).
	AdminWebBaseURL string
}

func Load() (Config, error) {
	cfg := Config{
		DBUrl:             getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress: getEnv("GRPC_SERVER_ADDRESS", ":9095"),
		HTTPServerAddress: getEnv("HTTP_SERVER_ADDRESS", ":8085"),

		AuthServiceAddress:  getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),
		StatsServiceAddress: getEnv("STATS_SERVICE_ADDRESS", ""),
		StudyServiceAddress: getEnv("STUDY_SERVICE_ADDRESS", ""),

		PubSubPushSecret: getEnv("PUBSUB_PUSH_SECRET", ""),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		EmailFrom:    getEnv("EMAIL_FROM", "no-reply@mempan.app"),

		FCMProjectID:       getEnv("FCM_PROJECT_ID", ""),
		FCMCredentialsFile: getEnv("FCM_CREDENTIALS_FILE", ""),

		AppBaseURL:      getEnv("APP_BASE_URL", "https://mempan.app"),
		AdminWebBaseURL: getEnv("ADMIN_WEB_BASE_URL", "http://localhost:5173"),
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

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
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
