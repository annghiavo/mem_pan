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

	PayOSClientID    string
	PayOSAPIKey      string
	PayOSChecksumKey string
	PayOSBaseURL     string

	PayOSPayoutClientID    string
	PayOSPayoutAPIKey      string
	PayOSPayoutChecksumKey string

	AppBaseURL       string
	DefaultReturnURL string
	DefaultCancelURL string

	PlusMonthlyAmountVND int64
	PlusYearlyAmountVND  int64
}

func Load() (Config, error) {
	appBaseURL := getEnv("APP_BASE_URL", "http://localhost:3000")
	cfg := Config{
		DBUrl:              getEnv("DATABASE_URL", firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))),
		GRPCServerAddress:  getEnv("GRPC_SERVER_ADDRESS", ":9098"),
		HTTPServerAddress:  getEnv("HTTP_SERVER_ADDRESS", ":8088"),
		AuthServiceAddress: getEnv("AUTH_SERVICE_ADDRESS", "localhost:9090"),

		PayOSClientID:    getEnv("PAYOS_CLIENT_ID", ""),
		PayOSAPIKey:      getEnv("PAYOS_API_KEY", ""),
		PayOSChecksumKey: getEnv("PAYOS_CHECKSUM_KEY", ""),
		PayOSBaseURL:     getEnv("PAYOS_BASE_URL", "https://api-merchant.payos.vn"),

		PayOSPayoutClientID:    getEnv("PAYOS_PAYOUT_CLIENT_ID", ""),
		PayOSPayoutAPIKey:      getEnv("PAYOS_PAYOUT_API_KEY", ""),
		PayOSPayoutChecksumKey: getEnv("PAYOS_PAYOUT_CHECKSUM_KEY", ""),

		AppBaseURL:       appBaseURL,
		DefaultReturnURL: getEnv("PAYOS_RETURN_URL", appBaseURL+"/billing/return"),
		DefaultCancelURL: getEnv("PAYOS_CANCEL_URL", appBaseURL+"/billing/cancel"),

		PlusMonthlyAmountVND: getEnvInt64("PLUS_MONTHLY_AMOUNT_VND", 49000),
		PlusYearlyAmountVND:  getEnvInt64("PLUS_YEARLY_AMOUNT_VND", 490000),
	}

	if cfg.DBUrl == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.PayOSClientID == "" {
		return Config{}, fmt.Errorf("PAYOS_CLIENT_ID is required")
	}
	if cfg.PayOSAPIKey == "" {
		return Config{}, fmt.Errorf("PAYOS_API_KEY is required")
	}
	if cfg.PayOSChecksumKey == "" {
		return Config{}, fmt.Errorf("PAYOS_CHECKSUM_KEY is required")
	}
	if cfg.PayOSPayoutClientID == "" {
		return Config{}, fmt.Errorf("PAYOS_PAYOUT_CLIENT_ID is required")
	}
	if cfg.PayOSPayoutAPIKey == "" {
		return Config{}, fmt.Errorf("PAYOS_PAYOUT_API_KEY is required")
	}
	if cfg.PayOSPayoutChecksumKey == "" {
		return Config{}, fmt.Errorf("PAYOS_PAYOUT_CHECKSUM_KEY is required")
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
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
