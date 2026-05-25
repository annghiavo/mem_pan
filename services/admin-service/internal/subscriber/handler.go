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
	"mem_pan/services/admin-service/internal/service"
)

type Handler struct {
	reportRepo repository.ReportRepository
	appealSvc  service.AppealService
}

func NewHandler(reportRepo repository.ReportRepository, appealSvc service.AppealService) *Handler {
	return &Handler{reportRepo: reportRepo, appealSvc: appealSvc}
}

func (h *Handler) Dispatch(ctx context.Context, eventType string, data []byte) error {
	switch eventType {
	case events.TypeReportSubmitted:
		return h.handleReportSubmitted(ctx, data)
	case events.TypeModerationDeckDeleted:
		return h.handleModerationDeckDeleted(ctx, data)
	default:
		log.Printf("[admin-subscriber] unknown event type %q — skipping", eventType)
		return nil
	}
}

// handleModerationDeckDeleted mints a deck-appeal row when moderation-fsrs
// auto-deletes a deck. The mint also publishes deck.appeal_available so
// notification-service emails the owner with the appeal link.
//
// Errors are logged but acked — notification-service will not retry email
// independently, and we don't want the auto-moderation event to be retried in
// a tight loop.
func (h *Handler) handleModerationDeckDeleted(ctx context.Context, data []byte) error {
	if h.appealSvc == nil {
		log.Printf("[admin-subscriber] moderation.deck_deleted: appeal service not configured — skipping")
		return nil
	}

	var e events.ModerationDeckDeleted
	if err := json.Unmarshal(data, &e); err != nil {
		return fmt.Errorf("unmarshal moderation.deck_deleted: %w", err)
	}

	deckID, err := uuid.Parse(e.DeckID)
	if err != nil {
		log.Printf("[admin-subscriber] moderation.deck_deleted: bad deck_id %q — skipping", e.DeckID)
		return nil
	}
	userID, err := uuid.Parse(e.UserID)
	if err != nil {
		log.Printf("[admin-subscriber] moderation.deck_deleted: bad user_id %q — skipping", e.UserID)
		return nil
	}

	reason := "Auto-moderation flagged your deck"
	if e.Reason != "" {
		reason = "Auto-moderation: " + e.Reason
	}

	if _, _, err := h.appealSvc.EnsureAppealForDeletedDeck(ctx, service.EnsureAppealParams{
		DeckID:           deckID,
		UserID:           userID,
		DeckName:         e.DeckName,
		ModerationReason: reason,
	}); err != nil {
		log.Printf("[admin-subscriber] ensure appeal failed deck=%s: %v (acking)", e.DeckID, err)
	}
	return nil
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
