package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/deckclient"
	"mem_pan/services/admin-service/internal/domain"
	"mem_pan/services/admin-service/internal/publisher"
	"mem_pan/services/admin-service/internal/repository"
)

// AppealDecision is the verdict a moderator returns on a submitted appeal.
type AppealDecision string

const (
	AppealDecisionApprove AppealDecision = "approve"
	AppealDecisionReject  AppealDecision = "reject"
)

type EnsureAppealParams struct {
	DeckID           uuid.UUID
	UserID           uuid.UUID
	DeckName         string
	ModerationReason string
}

type SubmitAppealParams struct {
	Token   string
	Message string
}

type ListAppealsParams struct {
	Limit        int32
	Offset       int32
	StatusFilter db.NullAppealStatus
}

type AppealsPage struct {
	Appeals []db.DeckAppeal
	Total   int64
}

type DecideAppealParams struct {
	AppealID uuid.UUID
	AdminID  uuid.UUID
	Decision AppealDecision
	Note     string
}

type AppealService interface {
	EnsureAppealForDeletedDeck(ctx context.Context, p EnsureAppealParams) (db.DeckAppeal, bool, error)
	GetAppealByToken(ctx context.Context, token string) (db.DeckAppeal, error)
	SubmitAppeal(ctx context.Context, p SubmitAppealParams) (db.DeckAppeal, error)
	ListAppeals(ctx context.Context, p ListAppealsParams) (AppealsPage, error)
	GetAppealByID(ctx context.Context, id uuid.UUID) (db.DeckAppeal, error)
	DecideAppeal(ctx context.Context, p DecideAppealParams) (db.DeckAppeal, error)
}

type appealService struct {
	repo       repository.AppealRepository
	reportRepo repository.ReportRepository
	deckClient deckclient.Client
	publisher  publisher.EventPublisher
}

func NewAppealService(
	repo repository.AppealRepository,
	reportRepo repository.ReportRepository,
	deckClient deckclient.Client,
	pub publisher.EventPublisher,
) AppealService {
	if pub == nil {
		pub = publisher.NewNoopPublisher()
	}
	return &appealService{
		repo:       repo,
		reportRepo: reportRepo,
		deckClient: deckClient,
		publisher:  pub,
	}
}

// EnsureAppealForDeletedDeck creates a pending appeal for the deck if none
// exists yet, and publishes deck.appeal_available so notification-service emails
// the owner with the appeal link. Returns (appeal, created=true) when a new row
// was inserted, or (existingAppeal, false) when one already existed.
//
// Idempotent: safe to call from multiple call sites for the same deck.
func (s *appealService) EnsureAppealForDeletedDeck(
	ctx context.Context, p EnsureAppealParams,
) (db.DeckAppeal, bool, error) {
	if p.DeckID == uuid.Nil || p.UserID == uuid.Nil {
		return db.DeckAppeal{}, false, fmt.Errorf("deck_id and user_id are required")
	}

	token, err := newAppealToken()
	if err != nil {
		return db.DeckAppeal{}, false, fmt.Errorf("generate token: %w", err)
	}

	deckName := p.DeckName
	if deckName == "" {
		deckName = "your deck"
	}

	appeal, err := s.repo.CreateDeckAppeal(ctx, db.CreateDeckAppealParams{
		Token:            token,
		DeckID:           p.DeckID,
		UserID:           p.UserID,
		DeckName:         deckName,
		ModerationReason: p.ModerationReason,
	})
	if errors.Is(err, domain.ErrAppealAlreadyExists) {
		// Another path (e.g. concurrent report + auto-mod) already minted one.
		// Return the existing row so the caller knows about it.
		existing, getErr := s.repo.GetDeckAppealByDeck(ctx, p.DeckID)
		if getErr != nil {
			return db.DeckAppeal{}, false, getErr
		}
		return existing, false, nil
	}
	if err != nil {
		return db.DeckAppeal{}, false, err
	}

	if pubErr := s.publisher.PublishDeckAppealAvailable(ctx, publisher.DeckAppealAvailableEvent{
		AppealToken:      appeal.Token,
		DeckID:           appeal.DeckID.String(),
		UserID:           appeal.UserID.String(),
		DeckName:         appeal.DeckName,
		ModerationReason: appeal.ModerationReason,
		CreatedAt:        appeal.CreatedAt,
	}); pubErr != nil {
		log.Printf("[appeal] publish deck.appeal_available: %v", pubErr)
	}

	return appeal, true, nil
}

func (s *appealService) GetAppealByToken(ctx context.Context, token string) (db.DeckAppeal, error) {
	if token == "" {
		return db.DeckAppeal{}, domain.ErrAppealNotFound
	}
	return s.repo.GetDeckAppealByToken(ctx, token)
}

func (s *appealService) SubmitAppeal(ctx context.Context, p SubmitAppealParams) (db.DeckAppeal, error) {
	if p.Token == "" {
		return db.DeckAppeal{}, domain.ErrAppealNotFound
	}
	arg := db.SubmitDeckAppealParams{Token: p.Token}
	if p.Message != "" {
		arg.UserMessage = sql.NullString{String: p.Message, Valid: true}
	}
	appeal, err := s.repo.SubmitDeckAppeal(ctx, arg)
	if errors.Is(err, domain.ErrAppealNotSubmittable) {
		// Either token unknown OR appeal already submitted/decided.
		existing, lookupErr := s.repo.GetDeckAppealByToken(ctx, p.Token)
		if lookupErr != nil {
			return db.DeckAppeal{}, domain.ErrAppealNotFound
		}
		return existing, domain.ErrAppealNotSubmittable
	}
	return appeal, err
}

func (s *appealService) ListAppeals(ctx context.Context, p ListAppealsParams) (AppealsPage, error) {
	appeals, err := s.repo.ListDeckAppeals(ctx, db.ListDeckAppealsParams{
		Limit:        p.Limit,
		Offset:       p.Offset,
		StatusFilter: p.StatusFilter,
	})
	if err != nil {
		return AppealsPage{}, err
	}
	total, err := s.repo.CountDeckAppeals(ctx, p.StatusFilter)
	if err != nil {
		return AppealsPage{}, err
	}
	return AppealsPage{Appeals: appeals, Total: total}, nil
}

func (s *appealService) GetAppealByID(ctx context.Context, id uuid.UUID) (db.DeckAppeal, error) {
	return s.repo.GetDeckAppealByID(ctx, id)
}

func (s *appealService) DecideAppeal(ctx context.Context, p DecideAppealParams) (db.DeckAppeal, error) {
	var newStatus db.AppealStatus
	switch p.Decision {
	case AppealDecisionApprove:
		newStatus = db.AppealStatusApproved
	case AppealDecisionReject:
		newStatus = db.AppealStatusRejected
	default:
		return db.DeckAppeal{}, domain.ErrInvalidAppealAction
	}

	// Take the side-effect (restore the deck) BEFORE flipping the appeal row.
	// If the deck-service call fails, the appeal stays in 'submitted' and the
	// admin can retry; better than reporting a fake outcome to the user.
	if p.Decision == AppealDecisionApprove {
		existing, err := s.repo.GetDeckAppealByID(ctx, p.AppealID)
		if err != nil {
			return db.DeckAppeal{}, err
		}
		if existing.Status != db.AppealStatusSubmitted {
			return db.DeckAppeal{}, domain.ErrAppealNotSubmittable
		}
		if _, _, err := s.deckClient.UpdateDeckStatus(ctx, existing.DeckID.String(), "active"); err != nil {
			return db.DeckAppeal{}, fmt.Errorf("restore deck: %w", err)
		}
	}

	arg := db.DecideDeckAppealParams{
		AppealID:  p.AppealID,
		Status:    newStatus,
		DecidedBy: uuid.NullUUID{UUID: p.AdminID, Valid: true},
	}
	if p.Note != "" {
		arg.DecisionNote = sql.NullString{String: p.Note, Valid: true}
	}
	appeal, err := s.repo.DecideDeckAppeal(ctx, arg)
	if err != nil {
		return db.DeckAppeal{}, err
	}

	// Publish decision event so notification-service can send the final email.
	if pubErr := s.publisher.PublishAppealDecided(ctx, publisher.AppealDecidedEvent{
		AppealID:     appeal.AppealID.String(),
		DeckID:       appeal.DeckID.String(),
		UserID:       appeal.UserID.String(),
		DeckName:     appeal.DeckName,
		Decision:     string(newStatus),
		DecisionNote: appeal.DecisionNote.String,
		DecidedAt:    appeal.DecidedAt.Time,
	}); pubErr != nil {
		log.Printf("[appeal] publish appeal.decided: %v", pubErr)
	}

	// Audit log.
	logMeta, _ := json.Marshal(map[string]any{
		"appeal_id": appeal.AppealID.String(),
		"decision":  string(newStatus),
	})
	if _, err := s.reportRepo.CreateModerationLog(ctx, db.CreateModerationLogParams{
		AdminID:    p.AdminID,
		Action:     "decide_appeal",
		TargetType: "deck",
		TargetID:   appeal.DeckID,
		Reason:     sql.NullString{String: p.Note, Valid: p.Note != ""},
		Metadata:   sql.NullString{String: string(logMeta), Valid: true},
	}); err != nil {
		log.Printf("[appeal] moderation log insert failed: %v", err)
	}

	return appeal, nil
}

// newAppealToken returns a URL-safe random string. 32 bytes ~ 256 bits of
// entropy, base64-url encoded to ~43 chars (fits the VARCHAR(80) column).
func newAppealToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
