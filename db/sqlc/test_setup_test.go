package sqlc

import (
	"bufio"
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

var (
	testDB      *sql.DB
	testQueries *Queries
	initOnce    sync.Once
	initErr     error
)

func setupTestDB() error {
	initOnce.Do(func() {
		for _, path := range []string{"app.env", "../app.env", "../../app.env"} {
			loadEnvFile(path)
		}

		dbURL := firstNonEmpty(
			os.Getenv("DB_URL"),
			os.Getenv("DIRECT_URL"),
			os.Getenv("DATABASE_URL"),
		)
		if dbURL == "" {
			initErr = sql.ErrConnDone
			return
		}

		db, err := sql.Open("postgres", dbURL)
		if err != nil {
			initErr = err
			return
		}
		if err := db.PingContext(context.Background()); err != nil {
			_ = db.Close()
			initErr = err
			return
		}

		testDB = db
		testQueries = New(db)
	})

	return initErr
}

func requireTestDB(t *testing.T) {
	t.Helper()
	err := setupTestDB()
	if err == sql.ErrConnDone {
		t.Skip("DB_URL, DIRECT_URL, or DATABASE_URL is required for integration tests")
	}
	require.NoError(t, err)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}
