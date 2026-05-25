package publisher

import (
	"context"
	"time"
)

// ReportResolvedEvent is fired by admin-service after an admin acts on a report.
// notification-service consumes it and emails every distinct reporter.
type ReportResolvedEvent struct {
	TargetType    string    `json:"target_type"` // user | deck
	TargetID      string    `json:"target_id"`
	TargetOwnerID string    `json:"target_owner_id"`
	Action        string    `json:"action"`      // ban_user | hide_deck | delete_deck | dismiss
	Resolution    string    `json:"resolution"`  // banned | deck_hidden | deck_deleted | ""
	ReporterIDs   []string  `json:"reporter_ids"`
	ResolvedAt    time.Time `json:"resolved_at"`
}

// DeckAppealAvailableEvent is fired by admin-service the first time a deck is
// deleted (auto-moderation or admin action). notification-service emails the
// deck owner with the appeal link.
type DeckAppealAvailableEvent struct {
	AppealToken      string    `json:"appeal_token"`
	DeckID           string    `json:"deck_id"`
	UserID           string    `json:"user_id"`
	DeckName         string    `json:"deck_name"`
	ModerationReason string    `json:"moderation_reason"`
	CreatedAt        time.Time `json:"created_at"`
}

// AppealDecidedEvent is fired by admin-service after a moderator approves or
// rejects an appeal. notification-service emails the deck owner with the final
// decision (no appeal link this time).
type AppealDecidedEvent struct {
	AppealID     string    `json:"appeal_id"`
	DeckID       string    `json:"deck_id"`
	UserID       string    `json:"user_id"`
	DeckName     string    `json:"deck_name"`
	Decision     string    `json:"decision"` // approved | rejected
	DecisionNote string    `json:"decision_note"`
	DecidedAt    time.Time `json:"decided_at"`
}

type EventPublisher interface {
	PublishReportResolved(ctx context.Context, event ReportResolvedEvent) error
	PublishDeckAppealAvailable(ctx context.Context, event DeckAppealAvailableEvent) error
	PublishAppealDecided(ctx context.Context, event AppealDecidedEvent) error
}

type noopPublisher struct{}

func NewNoopPublisher() EventPublisher { return &noopPublisher{} }

func (p *noopPublisher) PublishReportResolved(_ context.Context, _ ReportResolvedEvent) error {
	return nil
}

func (p *noopPublisher) PublishDeckAppealAvailable(_ context.Context, _ DeckAppealAvailableEvent) error {
	return nil
}

func (p *noopPublisher) PublishAppealDecided(_ context.Context, _ AppealDecidedEvent) error {
	return nil
}
