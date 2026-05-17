package publisher

import (
	"context"
	"time"
)

// ReportResolvedEvent is fired by admin-service after an admin acts on a report.
// notification-service consumes it and emails every distinct reporter.
type ReportResolvedEvent struct {
	TargetType  string    `json:"target_type"` // user | deck
	TargetID    string    `json:"target_id"`
	Action      string    `json:"action"`      // ban_user | hide_deck | delete_deck | dismiss
	Resolution  string    `json:"resolution"`  // banned | deck_hidden | deck_deleted | ""
	ReporterIDs []string  `json:"reporter_ids"`
	ResolvedAt  time.Time `json:"resolved_at"`
}

type EventPublisher interface {
	PublishReportResolved(ctx context.Context, event ReportResolvedEvent) error
}

type noopPublisher struct{}

func NewNoopPublisher() EventPublisher { return &noopPublisher{} }

func (p *noopPublisher) PublishReportResolved(_ context.Context, _ ReportResolvedEvent) error {
	return nil
}
