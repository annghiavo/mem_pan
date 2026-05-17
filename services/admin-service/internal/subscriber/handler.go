package subscriber

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/events"
	"mem_pan/services/admin-service/internal/repository"
)

type Handler struct {
	reportRepo repository.ReportRepository
}

func NewHandler(reportRepo repository.ReportRepository) *Handler {
	return &Handler{reportRepo: reportRepo}
}

func (h *Handler) Dispatch(ctx context.Context, eventType string, data []byte) error {
	switch eventType {
	case events.TypeReportSubmitted:
		return h.handleReportSubmitted(ctx, data)
	default:
		log.Printf("[admin-subscriber] unknown event type %q — skipping", eventType)
		return nil
	}
}

var allowedTargetTypes = map[string]db.ReportTargetType{
	"deck": db.ReportTargetType("deck"),
	"user": db.ReportTargetType("user"),
}

var allowedCategories = map[string]db.ReportCategory{
	"inappropriate_content": db.ReportCategoryInappropriateContent,
	"copyright_violation":   db.ReportCategoryCopyrightViolation,
	"spam":                  db.ReportCategorySpam,
	"harassment":            db.ReportCategoryHarassment,
	"misinformation":        db.ReportCategoryMisinformation,
	"other":                 db.ReportCategoryOther,
}

func (h *Handler) handleReportSubmitted(ctx context.Context, data []byte) error {
	var e events.ReportSubmitted
	if err := json.Unmarshal(data, &e); err != nil {
		return fmt.Errorf("unmarshal report.submitted: %w", err)
	}

	reporterID, err := uuid.Parse(e.ReporterID)
	if err != nil {
		return fmt.Errorf("invalid reporter_id: %w", err)
	}
	targetID, err := uuid.Parse(e.TargetID)
	if err != nil {
		return fmt.Errorf("invalid target_id: %w", err)
	}

	targetType, ok := allowedTargetTypes[e.TargetType]
	if !ok {
		return fmt.Errorf("invalid target_type %q", e.TargetType)
	}
	reasonCategory, ok := allowedCategories[e.ReasonCategory]
	if !ok {
		return fmt.Errorf("invalid reason_category %q", e.ReasonCategory)
	}

	arg := db.CreateReportParams{
		ReporterID:     reporterID,
		TargetType:     targetType,
		TargetID:       targetID,
		ReasonCategory: reasonCategory,
	}
	if e.Description != "" {
		arg.Description = sql.NullString{String: e.Description, Valid: true}
	}

	if _, err := h.reportRepo.CreateReport(ctx, arg); err != nil {
		return fmt.Errorf("insert report: %w", err)
	}
	return nil
}
