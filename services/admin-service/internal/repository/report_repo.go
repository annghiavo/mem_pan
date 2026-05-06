package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/domain"
)

type ReportRepository interface {
	CreateReport(ctx context.Context, arg db.CreateReportParams) (db.Report, error)
	GetReport(ctx context.Context, reportID uuid.UUID) (db.Report, error)
	ListReports(ctx context.Context, arg db.ListReportsParams) ([]db.Report, error)
	CountReports(ctx context.Context, statusFilter db.NullReportStatus) (int64, error)
	UpdateReportStatus(ctx context.Context, arg db.UpdateReportStatusParams) (db.Report, error)
	CreateModerationLog(ctx context.Context, arg db.CreateModerationLogParams) (db.ModerationLog, error)
}

type reportRepository struct {
	q *db.Queries
}

func NewReportRepository(database *sql.DB) ReportRepository {
	return &reportRepository{q: db.New(database)}
}

func (r *reportRepository) CreateReport(ctx context.Context, arg db.CreateReportParams) (db.Report, error) {
	return r.q.CreateReport(ctx, arg)
}

func (r *reportRepository) GetReport(ctx context.Context, reportID uuid.UUID) (db.Report, error) {
	report, err := r.q.GetReport(ctx, reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Report{}, domain.ErrReportNotFound
	}
	return report, err
}

func (r *reportRepository) ListReports(ctx context.Context, arg db.ListReportsParams) ([]db.Report, error) {
	return r.q.ListReports(ctx, arg)
}

func (r *reportRepository) CountReports(ctx context.Context, statusFilter db.NullReportStatus) (int64, error) {
	return r.q.CountReports(ctx, statusFilter)
}

func (r *reportRepository) UpdateReportStatus(ctx context.Context, arg db.UpdateReportStatusParams) (db.Report, error) {
	report, err := r.q.UpdateReportStatus(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Report{}, domain.ErrReportNotFound
	}
	return report, err
}

func (r *reportRepository) CreateModerationLog(ctx context.Context, arg db.CreateModerationLogParams) (db.ModerationLog, error) {
	return r.q.CreateModerationLog(ctx, arg)
}
