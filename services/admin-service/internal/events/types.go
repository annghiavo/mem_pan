package events

import "time"

// Event type constants — must match the publisher strings in the originating services.
const (
	// Published by deck-service and auth-service when a user files a moderation report.
	TypeReportSubmitted = "report.submitted"
)

// Envelope wraps every Pub/Sub message body.
type Envelope struct {
	EventType string `json:"event_type"`
	Data      []byte `json:"data"`
}

// ReportSubmitted is the payload of a "report.submitted" event.
// admin-service persists it to admin_db.reports.
type ReportSubmitted struct {
	ReporterID     string    `json:"reporter_id"`
	TargetType     string    `json:"target_type"`     // deck | user | note
	TargetID       string    `json:"target_id"`
	ReasonCategory string    `json:"reason_category"`
	Description    string    `json:"description"`
	SubmittedAt    time.Time `json:"submitted_at"`
}
