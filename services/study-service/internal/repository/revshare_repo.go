package repository

import (
	"context"
	"database/sql"
	"time"

	"mem_pan/services/study-service/internal/db"
)

type RevshareRepository interface {
	UpsertStudySessionMetrics(ctx context.Context, arg db.UpsertStudySessionMetricsParams) (db.StudySessionMetric, error)
	UpsertMonthlyRevenuePool(ctx context.Context, arg db.UpsertMonthlyRevenuePoolParams) (db.MonthlyRevenuePool, error)
	DeleteCreatorEarningsForMonth(ctx context.Context, poolMonth time.Time) error
	InsertCreatorEarningsForMonth(ctx context.Context, arg db.InsertCreatorEarningsForMonthParams) ([]db.CreatorEarning, error)
	FinalizeRevenuePool(ctx context.Context, poolMonth time.Time) (db.MonthlyRevenuePool, error)
	ListCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error)
}

type revshareRepository struct {
	q *db.Queries
}

func NewRevshareRepository(database *sql.DB) RevshareRepository {
	return &revshareRepository{q: db.New(database)}
}

func (r *revshareRepository) UpsertStudySessionMetrics(ctx context.Context, arg db.UpsertStudySessionMetricsParams) (db.StudySessionMetric, error) {
	return r.q.UpsertStudySessionMetrics(ctx, arg)
}

func (r *revshareRepository) UpsertMonthlyRevenuePool(ctx context.Context, arg db.UpsertMonthlyRevenuePoolParams) (db.MonthlyRevenuePool, error) {
	return r.q.UpsertMonthlyRevenuePool(ctx, arg)
}

func (r *revshareRepository) DeleteCreatorEarningsForMonth(ctx context.Context, poolMonth time.Time) error {
	return r.q.DeleteCreatorEarningsForMonth(ctx, poolMonth)
}

func (r *revshareRepository) InsertCreatorEarningsForMonth(ctx context.Context, arg db.InsertCreatorEarningsForMonthParams) ([]db.CreatorEarning, error) {
	return r.q.InsertCreatorEarningsForMonth(ctx, arg)
}

func (r *revshareRepository) FinalizeRevenuePool(ctx context.Context, poolMonth time.Time) (db.MonthlyRevenuePool, error) {
	return r.q.FinalizeRevenuePool(ctx, poolMonth)
}

func (r *revshareRepository) ListCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error) {
	return r.q.ListCreatorEarningsByMonth(ctx, poolMonth)
}
