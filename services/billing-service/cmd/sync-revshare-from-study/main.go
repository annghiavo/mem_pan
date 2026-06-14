package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	billingdb "mem_pan/services/billing-service/internal/db"
	"mem_pan/services/billing-service/internal/repository"
)

type sourcePool struct {
	PoolMonth            time.Time
	GrossAmountVnd       int64
	CreatorPoolAmountVnd int64
	PlatformAmountVnd    int64
	Status               string
	FinalizedAt          sql.NullTime
}

type sourceEarning struct {
	PoolMonth        time.Time
	CreatorID        uuid.UUID
	EligibleLearners int32
	WeightedScore    string
	AmountVnd        int64
	Status           string
}

func main() {
	var (
		sourceDBURL  = flag.String("source-database-url", firstNonEmpty(os.Getenv("STUDY_DATABASE_URL"), os.Getenv("SOURCE_DATABASE_URL")), "Source study-service Postgres connection string.")
		billingDBURL = flag.String("billing-database-url", firstNonEmpty(os.Getenv("BILLING_DATABASE_URL"), os.Getenv("DATABASE_URL"), os.Getenv("DB_URL"), os.Getenv("DIRECT_URL")), "Destination billing-service Postgres connection string.")
		month        = flag.String("month", "", "Only sync one pool month, as YYYY-MM or YYYY-MM-DD.")
		fromMonth    = flag.String("from", "", "Start pool month, inclusive, as YYYY-MM or YYYY-MM-DD.")
		toMonth      = flag.String("to", "", "End pool month, inclusive, as YYYY-MM or YYYY-MM-DD.")
	)
	flag.Parse()

	if *sourceDBURL == "" {
		log.Fatal("source database URL is required: set STUDY_DATABASE_URL or pass -source-database-url")
	}
	if *billingDBURL == "" {
		log.Fatal("billing database URL is required: set BILLING_DATABASE_URL or pass -billing-database-url")
	}
	filters, err := parseFilters(*month, *fromMonth, *toMonth)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sourceDB, err := openDB(*sourceDBURL)
	if err != nil {
		log.Fatal(err)
	}
	defer sourceDB.Close()

	billingDB, err := openDB(*billingDBURL)
	if err != nil {
		log.Fatal(err)
	}
	defer billingDB.Close()

	pools, err := loadPools(ctx, sourceDB, filters)
	if err != nil {
		log.Fatal(err)
	}
	repo := repository.New(billingDB)
	syncedEarnings := 0
	for _, pool := range pools {
		earnings, err := loadEarningsByMonth(ctx, sourceDB, pool.PoolMonth)
		if err != nil {
			log.Fatalf("load earnings for %s: %v", pool.PoolMonth.Format("2006-01-02"), err)
		}
		items := make([]billingdb.UpsertCreatorEarningParams, 0, len(earnings))
		for _, earning := range earnings {
			items = append(items, billingdb.UpsertCreatorEarningParams{
				PoolMonth:        earning.PoolMonth,
				CreatorID:        earning.CreatorID,
				EligibleLearners: earning.EligibleLearners,
				WeightedScore:    earning.WeightedScore,
				AmountVnd:        earning.AmountVnd,
				Status:           firstNonEmpty(strings.TrimSpace(earning.Status), "pending"),
			})
		}
		err = repo.SyncRevenuePool(ctx, billingdb.UpsertMonthlyRevenuePoolParams{
			PoolMonth:            pool.PoolMonth,
			GrossAmountVnd:       pool.GrossAmountVnd,
			CreatorPoolAmountVnd: pool.CreatorPoolAmountVnd,
			PlatformAmountVnd:    pool.PlatformAmountVnd,
			Status:               pool.Status,
			FinalizedAt:          pool.FinalizedAt,
		}, items)
		if err != nil {
			log.Fatalf("sync pool %s: %v", pool.PoolMonth.Format("2006-01-02"), err)
		}
		syncedEarnings += len(items)
		log.Printf("synced pool %s with %d earnings", pool.PoolMonth.Format("2006-01-02"), len(items))
	}

	log.Printf("done: synced %d pools and %d earnings", len(pools), syncedEarnings)
}

type filterRange struct {
	Month     string
	FromMonth string
	ToMonth   string
}

func parseFilters(month, fromMonth, toMonth string) (filterRange, error) {
	if month != "" && (fromMonth != "" || toMonth != "") {
		return filterRange{}, fmt.Errorf("use either -month or -from/-to, not both")
	}
	out := filterRange{}
	if month != "" {
		parsed, err := parseMonth(month)
		if err != nil {
			return filterRange{}, fmt.Errorf("invalid -month: %w", err)
		}
		out.Month = parsed
	}
	if fromMonth != "" {
		parsed, err := parseMonth(fromMonth)
		if err != nil {
			return filterRange{}, fmt.Errorf("invalid -from: %w", err)
		}
		out.FromMonth = parsed
	}
	if toMonth != "" {
		parsed, err := parseMonth(toMonth)
		if err != nil {
			return filterRange{}, fmt.Errorf("invalid -to: %w", err)
		}
		out.ToMonth = parsed
	}
	if out.FromMonth != "" && out.ToMonth != "" && out.FromMonth > out.ToMonth {
		return filterRange{}, fmt.Errorf("-from must be before or equal to -to")
	}
	return out, nil
}

func parseMonth(value string) (string, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01", "2006-01-02"} {
		t, err := time.Parse(layout, value)
		if err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("expected YYYY-MM or YYYY-MM-DD")
}

func loadPools(ctx context.Context, db *sql.DB, filters filterRange) ([]sourcePool, error) {
	var (
		args  []any
		where []string
	)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	where = append(where, "status = 'finalized'")
	if filters.Month != "" {
		where = append(where, "pool_month = "+addArg(filters.Month)+"::date")
	}
	if filters.FromMonth != "" {
		where = append(where, "pool_month >= "+addArg(filters.FromMonth)+"::date")
	}
	if filters.ToMonth != "" {
		where = append(where, "pool_month <= "+addArg(filters.ToMonth)+"::date")
	}

	query := `
SELECT pool_month, gross_amount_vnd, creator_pool_amount_vnd, platform_amount_vnd, status, finalized_at
FROM monthly_revenue_pools
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY pool_month`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pools []sourcePool
	for rows.Next() {
		var item sourcePool
		if err := rows.Scan(
			&item.PoolMonth,
			&item.GrossAmountVnd,
			&item.CreatorPoolAmountVnd,
			&item.PlatformAmountVnd,
			&item.Status,
			&item.FinalizedAt,
		); err != nil {
			return nil, err
		}
		pools = append(pools, item)
	}
	return pools, rows.Err()
}

func loadEarningsByMonth(ctx context.Context, db *sql.DB, poolMonth time.Time) ([]sourceEarning, error) {
	rows, err := db.QueryContext(ctx, `
SELECT pool_month, creator_id, eligible_learners, weighted_score::text, amount_vnd, status
FROM creator_earnings
WHERE pool_month = $1
ORDER BY creator_id
`, poolMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var earnings []sourceEarning
	for rows.Next() {
		var item sourceEarning
		if err := rows.Scan(
			&item.PoolMonth,
			&item.CreatorID,
			&item.EligibleLearners,
			&item.WeightedScore,
			&item.AmountVnd,
			&item.Status,
		); err != nil {
			return nil, err
		}
		earnings = append(earnings, item)
	}
	return earnings, rows.Err()
}

func openDB(databaseURL string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(2 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return db, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
