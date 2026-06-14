package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

const minimumWithdrawalVND = int64(100000)

type earningRow struct {
	EarningID                string     `json:"earning_id"`
	PoolMonth                string     `json:"pool_month"`
	PoolStatus               string     `json:"pool_status"`
	CreatorID                string     `json:"creator_id"`
	EligibleLearners         int        `json:"eligible_learners"`
	WeightedScore            string     `json:"weighted_score"`
	AmountVND                int64      `json:"amount_vnd"`
	Status                   string     `json:"status"`
	PaidAt                   *time.Time `json:"paid_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	PayoutReferenceID        string     `json:"payout_reference_id,omitempty"`
	PayoutToBin              string     `json:"payout_to_bin,omitempty"`
	PayoutToAccountNumber    string     `json:"payout_to_account_number,omitempty"`
	PayoutToAccountName      string     `json:"payout_to_account_name,omitempty"`
	PayOSPayoutID            string     `json:"payos_payout_id,omitempty"`
	PayOSPayoutTransactionID string     `json:"payos_payout_transaction_id,omitempty"`
	PayOSPayoutState         string     `json:"payos_payout_state,omitempty"`
	PayoutRequestedAt        *time.Time `json:"payout_requested_at,omitempty"`
	PayoutFailedReason       string     `json:"payout_failed_reason,omitempty"`
	GrossAmountVND           int64      `json:"gross_amount_vnd"`
	CreatorPoolAmountVND     int64      `json:"creator_pool_amount_vnd"`
	PlatformAmountVND        int64      `json:"platform_amount_vnd"`
	PoolFinalizedAt          *time.Time `json:"pool_finalized_at,omitempty"`
}

type reportSummary struct {
	Rows                    int              `json:"rows"`
	Creators                int              `json:"creators"`
	TotalAmountVND          int64            `json:"total_amount_vnd"`
	WithdrawableAmountVND   int64            `json:"withdrawable_amount_vnd"`
	ByStatusAmountVND       map[string]int64 `json:"by_status_amount_vnd"`
	ByStatusCount           map[string]int   `json:"by_status_count"`
	PoolCreatorAmountVND    map[string]int64 `json:"pool_creator_amount_vnd"`
	PoolGrossAmountVND      map[string]int64 `json:"pool_gross_amount_vnd"`
	PoolPlatformAmountVND   map[string]int64 `json:"pool_platform_amount_vnd"`
	TopCreatorsByAmountVND  []creatorSummary `json:"top_creators_by_amount_vnd"`
	MinimumWithdrawalAmount int64            `json:"minimum_withdrawal_amount_vnd"`
}

type creatorSummary struct {
	CreatorID string `json:"creator_id"`
	AmountVND int64  `json:"amount_vnd"`
	Rows      int    `json:"rows"`
}

type report struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Filters     reportFilters `json:"filters"`
	Summary     reportSummary `json:"summary"`
	Earnings    []earningRow  `json:"earnings"`
}

type reportFilters struct {
	Month     string `json:"month,omitempty"`
	FromMonth string `json:"from_month,omitempty"`
	ToMonth   string `json:"to_month,omitempty"`
	CreatorID string `json:"creator_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

func main() {
	var (
		databaseURL = flag.String("database-url", "", "Postgres connection string. Defaults to DATABASE_URL, DB_URL, or DIRECT_URL.")
		envFile     = flag.String("env-file", "app.env", "Optional env file to load before reading database config.")
		month       = flag.String("month", "", "Pool month to report, as YYYY-MM or YYYY-MM-DD.")
		fromMonth   = flag.String("from", "", "Start pool month, inclusive, as YYYY-MM or YYYY-MM-DD.")
		toMonth     = flag.String("to", "", "End pool month, inclusive, as YYYY-MM or YYYY-MM-DD.")
		creatorID   = flag.String("creator-id", "", "Filter by creator UUID.")
		status      = flag.String("status", "", "Filter by earning status, for example pending, processing, paid, or failed.")
		format      = flag.String("format", "table", "Output format: table, csv, or json.")
		limit       = flag.Int("limit", 0, "Maximum earning rows to return. 0 means no limit.")
	)
	flag.Parse()

	_ = godotenv.Load(*envFile)
	dbURL := firstNonEmpty(*databaseURL, os.Getenv("DATABASE_URL"), os.Getenv("DB_URL"), os.Getenv("DIRECT_URL"))
	if dbURL == "" {
		log.Fatal("database URL is required: set DATABASE_URL or pass -database-url")
	}

	filters, err := parseFilters(*month, *fromMonth, *toMonth, *creatorID, *status, *limit)
	if err != nil {
		log.Fatal(err)
	}

	db, err := openDB(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rows, err := loadEarnings(ctx, db, filters)
	if err != nil {
		log.Fatal(err)
	}

	out := report{
		GeneratedAt: time.Now().UTC(),
		Filters:     filters,
		Summary:     summarize(rows),
		Earnings:    rows,
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "table":
		if err := writeTable(os.Stdout, out); err != nil {
			log.Fatal(err)
		}
	case "csv":
		if err := writeCSV(os.Stdout, out.Earnings); err != nil {
			log.Fatal(err)
		}
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unsupported -format %q; use table, csv, or json", *format)
	}
}

func openDB(databaseURL string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(2 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return db, nil
}

func parseFilters(month, fromMonth, toMonth, creatorID, status string, limit int) (reportFilters, error) {
	if limit < 0 {
		return reportFilters{}, fmt.Errorf("-limit must be >= 0")
	}
	if month != "" && (fromMonth != "" || toMonth != "") {
		return reportFilters{}, fmt.Errorf("use either -month or -from/-to, not both")
	}
	out := reportFilters{
		CreatorID: strings.TrimSpace(creatorID),
		Status:    strings.ToLower(strings.TrimSpace(status)),
		Limit:     limit,
	}
	if month != "" {
		parsed, err := parseMonth(month)
		if err != nil {
			return reportFilters{}, fmt.Errorf("invalid -month: %w", err)
		}
		out.Month = parsed
	}
	if fromMonth != "" {
		parsed, err := parseMonth(fromMonth)
		if err != nil {
			return reportFilters{}, fmt.Errorf("invalid -from: %w", err)
		}
		out.FromMonth = parsed
	}
	if toMonth != "" {
		parsed, err := parseMonth(toMonth)
		if err != nil {
			return reportFilters{}, fmt.Errorf("invalid -to: %w", err)
		}
		out.ToMonth = parsed
	}
	if out.FromMonth != "" && out.ToMonth != "" && out.FromMonth > out.ToMonth {
		return reportFilters{}, fmt.Errorf("-from must be before or equal to -to")
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

func loadEarnings(ctx context.Context, db *sql.DB, filters reportFilters) ([]earningRow, error) {
	var args []any
	var where []string
	addArg := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}

	if filters.Month != "" {
		where = append(where, "ce.pool_month = "+addArg(filters.Month)+"::date")
	}
	if filters.FromMonth != "" {
		where = append(where, "ce.pool_month >= "+addArg(filters.FromMonth)+"::date")
	}
	if filters.ToMonth != "" {
		where = append(where, "ce.pool_month <= "+addArg(filters.ToMonth)+"::date")
	}
	if filters.CreatorID != "" {
		where = append(where, "ce.creator_id = "+addArg(filters.CreatorID)+"::uuid")
	}
	if filters.Status != "" {
		where = append(where, "LOWER(ce.status) = "+addArg(filters.Status))
	}

	query := `
SELECT
    ce.earning_id::text,
    ce.pool_month,
    mrp.status AS pool_status,
    ce.creator_id::text,
    ce.eligible_learners,
    ce.weighted_score::text,
    ce.amount_vnd,
    ce.status,
    ce.paid_at,
    ce.created_at,
    ce.payout_reference_id,
    ce.payout_to_bin,
    ce.payout_to_account_number,
    ce.payout_to_account_name,
    ce.payos_payout_id,
    ce.payos_payout_transaction_id,
    ce.payos_payout_state,
    ce.payout_requested_at,
    ce.payout_failed_reason,
    mrp.gross_amount_vnd,
    mrp.creator_pool_amount_vnd,
    mrp.platform_amount_vnd,
    mrp.finalized_at
FROM creator_earnings ce
JOIN monthly_revenue_pools mrp ON mrp.pool_month = ce.pool_month`
	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, "\n  AND ")
	}
	query += "\nORDER BY ce.pool_month DESC, ce.amount_vnd DESC, ce.creator_id"
	if filters.Limit > 0 {
		query += "\nLIMIT " + addArg(filters.Limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query creator earnings: %w", err)
	}
	defer rows.Close()

	var out []earningRow
	for rows.Next() {
		var (
			row                      earningRow
			poolMonth                time.Time
			paidAt                   sql.NullTime
			payoutReferenceID        sql.NullString
			payoutToBin              sql.NullString
			payoutToAccountNumber    sql.NullString
			payoutToAccountName      sql.NullString
			payOSPayoutID            sql.NullString
			payOSPayoutTransactionID sql.NullString
			payOSPayoutState         sql.NullString
			payoutRequestedAt        sql.NullTime
			payoutFailedReason       sql.NullString
			poolFinalizedAt          sql.NullTime
		)
		err := rows.Scan(
			&row.EarningID,
			&poolMonth,
			&row.PoolStatus,
			&row.CreatorID,
			&row.EligibleLearners,
			&row.WeightedScore,
			&row.AmountVND,
			&row.Status,
			&paidAt,
			&row.CreatedAt,
			&payoutReferenceID,
			&payoutToBin,
			&payoutToAccountNumber,
			&payoutToAccountName,
			&payOSPayoutID,
			&payOSPayoutTransactionID,
			&payOSPayoutState,
			&payoutRequestedAt,
			&payoutFailedReason,
			&row.GrossAmountVND,
			&row.CreatorPoolAmountVND,
			&row.PlatformAmountVND,
			&poolFinalizedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan creator earning: %w", err)
		}
		row.PoolMonth = poolMonth.Format("2006-01-02")
		row.PaidAt = nullTimePtr(paidAt)
		row.PayoutReferenceID = nullString(payoutReferenceID)
		row.PayoutToBin = nullString(payoutToBin)
		row.PayoutToAccountNumber = nullString(payoutToAccountNumber)
		row.PayoutToAccountName = nullString(payoutToAccountName)
		row.PayOSPayoutID = nullString(payOSPayoutID)
		row.PayOSPayoutTransactionID = nullString(payOSPayoutTransactionID)
		row.PayOSPayoutState = nullString(payOSPayoutState)
		row.PayoutRequestedAt = nullTimePtr(payoutRequestedAt)
		row.PayoutFailedReason = nullString(payoutFailedReason)
		row.PoolFinalizedAt = nullTimePtr(poolFinalizedAt)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read creator earnings: %w", err)
	}
	return out, nil
}

func summarize(rows []earningRow) reportSummary {
	creators := make(map[string]struct{})
	byStatusAmount := make(map[string]int64)
	byStatusCount := make(map[string]int)
	poolCreator := make(map[string]int64)
	poolGross := make(map[string]int64)
	poolPlatform := make(map[string]int64)
	byCreator := make(map[string]creatorSummary)

	var total int64
	var withdrawable int64
	for _, row := range rows {
		creators[row.CreatorID] = struct{}{}
		total += row.AmountVND
		status := strings.ToLower(row.Status)
		byStatusAmount[status] += row.AmountVND
		byStatusCount[status]++
		poolCreator[row.PoolMonth] = row.CreatorPoolAmountVND
		poolGross[row.PoolMonth] = row.GrossAmountVND
		poolPlatform[row.PoolMonth] = row.PlatformAmountVND
		if (status == "pending" || status == "failed") && row.AmountVND > minimumWithdrawalVND {
			withdrawable += row.AmountVND
		}
		item := byCreator[row.CreatorID]
		item.CreatorID = row.CreatorID
		item.AmountVND += row.AmountVND
		item.Rows++
		byCreator[row.CreatorID] = item
	}

	topCreators := make([]creatorSummary, 0, len(byCreator))
	for _, item := range byCreator {
		topCreators = append(topCreators, item)
	}
	sort.Slice(topCreators, func(i, j int) bool {
		if topCreators[i].AmountVND == topCreators[j].AmountVND {
			return topCreators[i].CreatorID < topCreators[j].CreatorID
		}
		return topCreators[i].AmountVND > topCreators[j].AmountVND
	})
	if len(topCreators) > 10 {
		topCreators = topCreators[:10]
	}

	return reportSummary{
		Rows:                    len(rows),
		Creators:                len(creators),
		TotalAmountVND:          total,
		WithdrawableAmountVND:   withdrawable,
		ByStatusAmountVND:       byStatusAmount,
		ByStatusCount:           byStatusCount,
		PoolCreatorAmountVND:    poolCreator,
		PoolGrossAmountVND:      poolGross,
		PoolPlatformAmountVND:   poolPlatform,
		TopCreatorsByAmountVND:  topCreators,
		MinimumWithdrawalAmount: minimumWithdrawalVND,
	}
}

func writeTable(w io.Writer, out report) error {
	fmt.Fprintf(w, "Generated: %s\n", out.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Rows: %d\tCreators: %d\tTotal: %s\tWithdrawable: %s\n",
		out.Summary.Rows,
		out.Summary.Creators,
		formatVND(out.Summary.TotalAmountVND),
		formatVND(out.Summary.WithdrawableAmountVND),
	)
	if len(out.Summary.ByStatusAmountVND) > 0 {
		fmt.Fprintln(w, "\nStatus totals:")
		statuses := sortedKeys(out.Summary.ByStatusAmountVND)
		for _, status := range statuses {
			fmt.Fprintf(w, "  %s: %s (%d rows)\n", status, formatVND(out.Summary.ByStatusAmountVND[status]), out.Summary.ByStatusCount[status])
		}
	}
	if len(out.Summary.TopCreatorsByAmountVND) > 0 {
		fmt.Fprintln(w, "\nTop creators:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "CREATOR_ID\tAMOUNT\tROWS")
		for _, creator := range out.Summary.TopCreatorsByAmountVND {
			fmt.Fprintf(tw, "%s\t%s\t%d\n", creator.CreatorID, formatVND(creator.AmountVND), creator.Rows)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	if len(out.Earnings) == 0 {
		fmt.Fprintln(w, "\nNo creator earnings found for these filters.")
		return nil
	}

	fmt.Fprintln(w, "\nEarnings:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MONTH\tCREATOR_ID\tAMOUNT\tSTATUS\tLEARNERS\tSCORE\tPAYOUT_STATE\tREQUESTED_AT\tPAID_AT")
	for _, row := range out.Earnings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			row.PoolMonth,
			row.CreatorID,
			formatVND(row.AmountVND),
			row.Status,
			row.EligibleLearners,
			row.WeightedScore,
			defaultString(row.PayOSPayoutState, "-"),
			formatOptionalTime(row.PayoutRequestedAt),
			formatOptionalTime(row.PaidAt),
		)
	}
	return tw.Flush()
}

func writeCSV(w io.Writer, rows []earningRow) error {
	cw := csv.NewWriter(w)
	header := []string{
		"earning_id",
		"pool_month",
		"pool_status",
		"creator_id",
		"eligible_learners",
		"weighted_score",
		"amount_vnd",
		"status",
		"paid_at",
		"created_at",
		"payout_reference_id",
		"payout_to_bin",
		"payout_to_account_number",
		"payout_to_account_name",
		"payos_payout_id",
		"payos_payout_transaction_id",
		"payos_payout_state",
		"payout_requested_at",
		"payout_failed_reason",
		"gross_amount_vnd",
		"creator_pool_amount_vnd",
		"platform_amount_vnd",
		"pool_finalized_at",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		record := []string{
			row.EarningID,
			row.PoolMonth,
			row.PoolStatus,
			row.CreatorID,
			strconv.Itoa(row.EligibleLearners),
			row.WeightedScore,
			strconv.FormatInt(row.AmountVND, 10),
			row.Status,
			formatOptionalTime(row.PaidAt),
			row.CreatedAt.Format(time.RFC3339),
			row.PayoutReferenceID,
			row.PayoutToBin,
			row.PayoutToAccountNumber,
			row.PayoutToAccountName,
			row.PayOSPayoutID,
			row.PayOSPayoutTransactionID,
			row.PayOSPayoutState,
			formatOptionalTime(row.PayoutRequestedAt),
			row.PayoutFailedReason,
			strconv.FormatInt(row.GrossAmountVND, 10),
			strconv.FormatInt(row.CreatorPoolAmountVND, 10),
			strconv.FormatInt(row.PlatformAmountVND, 10),
			formatOptionalTime(row.PoolFinalizedAt),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func sortedKeys(values map[string]int64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatVND(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	s := strconv.FormatInt(value, 10)
	var parts []string
	for len(s) > 3 {
		parts = append(parts, s[len(s)-3:])
		s = s[:len(s)-3]
	}
	parts = append(parts, s)
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return sign + strings.Join(parts, ".") + " VND"
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
