package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/admin-service/internal/authclient"
	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/deckclient"
	"mem_pan/services/admin-service/internal/domain"
	"mem_pan/services/admin-service/internal/publisher"
	"mem_pan/services/admin-service/internal/repository"
)

// ProcessAction is the verb an admin picks when triaging a report.
type ProcessAction string

const (
	ActionBanUser    ProcessAction = "ban_user"
	ActionHideDeck   ProcessAction = "hide_deck"
	ActionDeleteDeck ProcessAction = "delete_deck"
	ActionDismiss    ProcessAction = "dismiss"
)

// Resolution stamped onto every report row affected by an action.
const (
	ResolutionBanned      = "banned"
	ResolutionDeckHidden  = "deck_hidden"
	ResolutionDeckDeleted = "deck_deleted"
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
	ReportID  uuid.UUID
	AdminID   uuid.UUID
	Action    ProcessAction
	AdminNote *string
}

type ProcessReportResult struct {
	Report             db.Report
	AffectedReports    int32
	NotifiedReporters  int32
}

type ReportService interface {
	ListReports(ctx context.Context, p ListReportsParams) (ReportsPage, error)
	GetReport(ctx context.Context, reportID uuid.UUID) (db.Report, error)
	ProcessReport(ctx context.Context, p ProcessReportParams) (ProcessReportResult, error)
}

type reportService struct {
	reportRepo    repository.ReportRepository
	authClient    authclient.Client
	deckClient    deckclient.Client
	publisher     publisher.EventPublisher
	appealService AppealService
}

func NewReportService(
	reportRepo repository.ReportRepository,
	authClient authclient.Client,
	deckClient deckclient.Client,
	pub publisher.EventPublisher,
	appealSvc AppealService,
) ReportService {
	if pub == nil {
		pub = publisher.NewNoopPublisher()
	}
	return &reportService{
		reportRepo:    reportRepo,
		authClient:    authClient,
		deckClient:    deckClient,
		publisher:     pub,
		appealService: appealSvc,
	}
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

// ProcessReport performs the moderation action implied by p.Action:
//   - bans the user / hides the deck / deletes the deck / dismisses
//
// All pending reports against the same target are resolved in one batch so the
// admin doesn't need to triage duplicates. The distinct reporter list is
// published as report.resolved for notification-service to email.
func (s *reportService) ProcessReport(ctx context.Context, p ProcessReportParams) (ProcessReportResult, error) {
	target, err := s.reportRepo.GetReportTarget(ctx, p.ReportID)
	if err != nil {
		return ProcessReportResult{}, err
	}

	resolution, newStatus, err := validateAction(p.Action, string(target.TargetType))
	if err != nil {
		return ProcessReportResult{}, err
	}

	// Take the moderation side-effect FIRST. If it fails, the reports stay pending
	// so the admin can retry; better than reporting a fake outcome to the reporters.
	ownerID, err := s.applyAction(ctx, p.Action, target)
	if err != nil {
		return ProcessReportResult{}, fmt.Errorf("apply moderation action: %w", err)
	}

	// Bulk-resolve every pending report against this same target.
	arg := db.BulkResolveReportsByTargetParams{
		TargetType: target.TargetType,
		TargetID:   target.TargetID,
		Status:     newStatus,
		ResolvedBy: uuid.NullUUID{UUID: p.AdminID, Valid: true},
	}
	if resolution != "" {
		arg.Resolution = sql.NullString{String: resolution, Valid: true}
	}
	if p.AdminNote != nil && *p.AdminNote != "" {
		arg.AdminNote = sql.NullString{String: *p.AdminNote, Valid: true}
	}
	affected, err := s.reportRepo.BulkResolveReportsByTarget(ctx, arg)
	if err != nil {
		return ProcessReportResult{}, fmt.Errorf("bulk resolve reports: %w", err)
	}
	if len(affected) == 0 {
		return ProcessReportResult{}, domain.ErrReportNotFound
	}

	primary := affected[0]
	for _, r := range affected {
		if r.ReportID == p.ReportID {
			primary = r
			break
		}
	}

	// Collect distinct reporters (across all reports for this target, not just affected) and publish.
	reporterIDs, err := s.reportRepo.ListReporterIDsByTarget(ctx, db.ListReporterIDsByTargetParams{
		TargetType: target.TargetType,
		TargetID:   target.TargetID,
	})
	if err != nil {
		log.Printf("[report] list reporters failed: %v", err)
	} else if pubErr := s.publisher.PublishReportResolved(ctx, publisher.ReportResolvedEvent{
		TargetType:    string(target.TargetType),
		TargetID:      target.TargetID.String(),
		TargetOwnerID: ownerID,
		Action:        string(p.Action),
		Resolution:    resolution,
		ReporterIDs:   uuidsToStrings(reporterIDs),
		ResolvedAt:    time.Now().UTC(),
	}); pubErr != nil {
		log.Printf("[publisher] report.resolved: %v", pubErr)
	}

	// Audit trail.
	logMeta, _ := json.Marshal(map[string]any{
		"action":          string(p.Action),
		"resolution":      resolution,
		"affected_count":  len(affected),
		"reporter_count":  len(reporterIDs),
	})
	if _, err := s.reportRepo.CreateModerationLog(ctx, db.CreateModerationLogParams{
		AdminID:    p.AdminID,
		Action:     string(p.Action),
		TargetType: string(target.TargetType),
		TargetID:   target.TargetID,
		Reason:     sqlNullStrFromPtr(p.AdminNote),
		Metadata:   sql.NullString{String: string(logMeta), Valid: true},
	}); err != nil {
		log.Printf("[report] moderation log insert failed: %v", err)
	}

	return ProcessReportResult{
		Report:            primary,
		AffectedReports:   int32(len(affected)),
		NotifiedReporters: int32(len(reporterIDs)),
	}, nil
}

// validateAction returns the resolution string + final report status for the action,
// and rejects mismatches like ban_user on a deck target.
func validateAction(action ProcessAction, targetType string) (resolution string, newStatus db.ReportStatus, err error) {
	switch action {
	case ActionBanUser:
		if targetType != "user" {
			return "", "", fmt.Errorf("ban_user only valid for user reports, got %s", targetType)
		}
		return ResolutionBanned, db.ReportStatusResolved, nil
	case ActionHideDeck:
		if targetType != "deck" {
			return "", "", fmt.Errorf("hide_deck only valid for deck reports, got %s", targetType)
		}
		return ResolutionDeckHidden, db.ReportStatusResolved, nil
	case ActionDeleteDeck:
		if targetType != "deck" {
			return "", "", fmt.Errorf("delete_deck only valid for deck reports, got %s", targetType)
		}
		return ResolutionDeckDeleted, db.ReportStatusResolved, nil
	case ActionDismiss:
		return "", db.ReportStatusDismissed, nil
	default:
		return "", "", fmt.Errorf("unknown action %q", action)
	}
}

func (s *reportService) applyAction(ctx context.Context, action ProcessAction, target db.GetReportTargetRow) (string, error) {
	switch action {
	case ActionBanUser:
		_, err := s.authClient.BanUser(ctx, target.TargetID, true, "moderation: report resolved")
		return target.TargetID.String(), err
	case ActionHideDeck:
		_, ownerID, err := s.deckClient.UpdateDeckStatus(ctx, target.TargetID.String(), "hidden")
		return ownerID, err
	case ActionDeleteDeck:
		deckName, ownerID, err := s.deckClient.UpdateDeckStatus(ctx, target.TargetID.String(), "deleted")
		if err != nil {
			return ownerID, err
		}
		s.ensureAppeal(ctx, target.TargetID, ownerID, deckName)
		return ownerID, nil
	case ActionDismiss:
		return "", nil
	default:
		return "", errors.New("unknown action")
	}
}

// ensureAppeal mints a deck-appeal row + email to the owner. Best-effort: logs
// on failure but does not roll back the moderation action — the deck has
// already been deleted.
func (s *reportService) ensureAppeal(ctx context.Context, deckID uuid.UUID, ownerIDStr, deckName string) {
	if s.appealService == nil || ownerIDStr == "" {
		return
	}
	ownerID, err := uuid.Parse(ownerIDStr)
	if err != nil {
		log.Printf("[report] cannot mint appeal: bad owner_id %q: %v", ownerIDStr, err)
		return
	}
	if _, _, err := s.appealService.EnsureAppealForDeletedDeck(ctx, EnsureAppealParams{
		DeckID:           deckID,
		UserID:           ownerID,
		DeckName:         deckName,
		ModerationReason: "Reported and removed by a moderator",
	}); err != nil {
		log.Printf("[report] ensure appeal failed deck=%s: %v", deckID, err)
	}
}

func uuidsToStrings(in []uuid.UUID) []string {
	out := make([]string, len(in))
	for i, u := range in {
		out[i] = u.String()
	}
	return out
}

func sqlNullStrFromPtr(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
