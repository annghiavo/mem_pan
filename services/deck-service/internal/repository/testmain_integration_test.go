//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testDB is the package-wide sql.DB pointing at the containerised Postgres
// that TestMain spins up. All repository tests in this package share it and
// must call truncateAll(t) at the top of each subtest to keep dataset isolated.
var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("deck_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("postgres container: %v", err)
	}
	defer func() {
		// Use a fresh context — `ctx` may already be cancelled by m.Run().
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		if err := pgC.Terminate(termCtx); err != nil {
			log.Printf("terminate container: %v", err)
		}
	}()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("connection string: %v", err)
	}

	// Resolve absolute path to db/migration from the location of this test
	// file so `go test` works regardless of the caller cwd.
	_, thisFile, _, _ := runtime.Caller(0)
	migrationsURL := "file://" + filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migration")

	migrator, err := migrate.New(migrationsURL, dsn)
	if err != nil {
		log.Fatalf("migrate.New: %v", err)
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migrate up: %v", err)
	}
	if srcErr, dbErr := migrator.Close(); srcErr != nil || dbErr != nil {
		log.Fatalf("migrator close: src=%v db=%v", srcErr, dbErr)
	}

	testDB, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("sql.Open: %v", err)
	}
	testDB.SetMaxOpenConns(10)
	if err := testDB.PingContext(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	code := m.Run()
	_ = testDB.Close()
	os.Exit(code)
}

// truncateAll resets every table to a known-empty state without touching the
// schema_migrations bookkeeping table. Call this at the start of each test.
func truncateAll(tb testing.TB) {
	tb.Helper()
	const stmt = `TRUNCATE TABLE folder_decks, cards, notes, decks, folders RESTART IDENTITY CASCADE`
	if _, err := testDB.ExecContext(context.Background(), stmt); err != nil {
		tb.Fatalf("truncate: %v", err)
	}
}

// debugDSN is exported only for ad-hoc debugging during test failures.
func debugDSN() string {
	if testDB == nil {
		return "(uninitialised)"
	}
	return fmt.Sprintf("conns=%d", testDB.Stats().OpenConnections)
}
