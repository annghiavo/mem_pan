package events

import "time"

// Event type constants — must match the publisher strings in the originating services.
const (
	// Published by deck-service and auth-service when a user files a moderation report.
	TypeReportSubmitted = "report.submitted"

	// Published by moderation-fsrs-service when an AI moderator auto-deletes a deck.
	// admin-service consumes it to create a deck_appeals row so the owner can contest.
	TypeModerationDeckDeleted = "moderation.deck_deleted"
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

// ModerationDeckDeleted mirrors the payload produced by moderation-fsrs-service.
// admin-service uses it to mint a deck-appeal row.
type ModerationDeckDeleted struct {
	DeckID           string    `json:"deck_id"`
	UserID           string    `json:"user_id"`
	DeckName         string    `json:"deck_name"`
	Reason           string    `json:"reason"`
	ViolatedCardIDs  []string  `json:"violated_card_ids"`
	DeletedAt        time.Time `json:"deleted_at"`
	ModeratorVersion string    `json:"moderator_version"`
}
