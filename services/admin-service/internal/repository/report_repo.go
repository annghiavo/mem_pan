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
	GetReportTarget(ctx context.Context, reportID uuid.UUID) (db.GetReportTargetRow, error)
	ListReports(ctx context.Context, arg db.ListReportsParams) ([]db.Report, error)
	CountReports(ctx context.Context, statusFilter db.NullReportStatus) (int64, error)
	BulkResolveReportsByTarget(ctx context.Context, arg db.BulkResolveReportsByTargetParams) ([]db.Report, error)
	ListReporterIDsByTarget(ctx context.Context, arg db.ListReporterIDsByTargetParams) ([]uuid.UUID, error)
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

func (r *reportRepository) GetReportTarget(ctx context.Context, reportID uuid.UUID) (db.GetReportTargetRow, error) {
	row, err := r.q.GetReportTarget(ctx, reportID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.GetReportTargetRow{}, domain.ErrReportNotFound
	}
	return row, err
}

func (r *reportRepository) ListReports(ctx context.Context, arg db.ListReportsParams) ([]db.Report, error) {
	return r.q.ListReports(ctx, arg)
}

func (r *reportRepository) CountReports(ctx context.Context, statusFilter db.NullReportStatus) (int64, error) {
	return r.q.CountReports(ctx, statusFilter)
}

func (r *reportRepository) BulkResolveReportsByTarget(ctx context.Context, arg db.BulkResolveReportsByTargetParams) ([]db.Report, error) {
	return r.q.BulkResolveReportsByTarget(ctx, arg)
}

func (r *reportRepository) ListReporterIDsByTarget(ctx context.Context, arg db.ListReporterIDsByTargetParams) ([]uuid.UUID, error) {
	return r.q.ListReporterIDsByTarget(ctx, arg)
}

func (r *reportRepository) CreateModerationLog(ctx context.Context, arg db.CreateModerationLogParams) (db.ModerationLog, error) {
	return r.q.CreateModerationLog(ctx, arg)
}
