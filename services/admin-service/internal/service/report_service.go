package service

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/repository"
)

type ListReportsParams struct {
	Limit        int32
	Offset       int32
	StatusFilter db.NullReportStatus
}

type ReportsPage struct {
	Reports []db.Report
	Total   int64
}

type ProcessReportParams struct {
	ReportID   uuid.UUID
	AdminID    uuid.UUID
	Status     db.ReportStatus
	AdminNote  *string
	Resolution *string
}

type ReportService interface {
	ListReports(ctx context.Context, p ListReportsParams) (ReportsPage, error)
	GetReport(ctx context.Context, reportID uuid.UUID) (db.Report, error)
	ProcessReport(ctx context.Context, p ProcessReportParams) (db.Report, error)
}

type reportService struct {
	reportRepo repository.ReportRepository
}

func NewReportService(reportRepo repository.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) ListReports(ctx context.Context, p ListReportsParams) (ReportsPage, error) {
	reports, err := s.reportRepo.ListReports(ctx, db.ListReportsParams{
		Limit:        p.Limit,
		Offset:       p.Offset,
		StatusFilter: p.StatusFilter,
	})
	if err != nil {
		return ReportsPage{}, err
	}
	total, err := s.reportRepo.CountReports(ctx, p.StatusFilter)
	if err != nil {
		return ReportsPage{}, err
	}
	return ReportsPage{Reports: reports, Total: total}, nil
}

func (s *reportService) GetReport(ctx context.Context, reportID uuid.UUID) (db.Report, error) {
	return s.reportRepo.GetReport(ctx, reportID)
}

func (s *reportService) ProcessReport(ctx context.Context, p ProcessReportParams) (db.Report, error) {
	arg := db.UpdateReportStatusParams{
		ReportID:   p.ReportID,
		Status:     p.Status,
		ResolvedBy: uuid.NullUUID{UUID: p.AdminID, Valid: true},
	}
	if p.AdminNote != nil {
		arg.AdminNote = sql.NullString{String: *p.AdminNote, Valid: true}
	}
	if p.Resolution != nil {
		arg.Resolution = sql.NullString{String: *p.Resolution, Valid: true}
	}

	report, err := s.reportRepo.UpdateReportStatus(ctx, arg)
	if err != nil {
		return db.Report{}, err
	}

	_, _ = s.reportRepo.CreateModerationLog(ctx, db.CreateModerationLogParams{
		AdminID:    p.AdminID,
		Action:     "process_report",
		TargetType: "report",
		TargetID:   p.ReportID,
		Reason:     sql.NullString{String: string(p.Status), Valid: true},
	})

	return report, nil
}
