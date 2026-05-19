package events

import "time"

// Event type constants — must match the publisher strings in each originating service.
const (
	// Published by auth-service
	TypeUserRegistered              = "user.registered"
	TypeEmailVerificationRequested  = "email.verification_requested"
	TypePasswordResetRequested      = "password.reset_requested"

	// Published by worker-service
	TypeDeckCloneCompleted = "deck.clone_completed"

	// Published by admin-service after an admin resolves/dismisses a report.
	TypeReportResolved = "report.resolved"

	// Published by Cloud Scheduler → Pub/Sub topic `cron-study-reminder`
	// every 15 minutes. The handler iterates eligible users and sends an
	// FCM push "you have N cards to review".
	TypeCronStudyReminder = "cron.study_reminder"

	// Published by Cloud Scheduler → Pub/Sub topic `cron-streak-warning`
	// every 15 minutes. The handler sends a push to users whose streak is
	// at risk (haven't studied today and local time is at their warning hour).
	TypeCronStreakWarning = "cron.streak_warning"
)

type Envelope struct {
	EventType string `json:"event_type"`
	Data      []byte `json:"data"`
}

// UserRegistered is published by auth-service on successful registration.
type UserRegistered struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

// EmailVerificationRequested is published by auth-service when a user needs to verify their email.
type EmailVerificationRequested struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PasswordResetRequested is published by auth-service when a user requests a password reset.
type PasswordResetRequested struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"reset_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DeckCloneCompleted is published by worker-service when a deck clone finishes.
type DeckCloneCompleted struct {
	UserID    string    `json:"user_id"`
	DeckID    string    `json:"deck_id"`
	DeckName  string    `json:"deck_name"`
	CardCount int32     `json:"card_count"`
	CreatedAt time.Time `json:"created_at"`
}

// ReportResolved is published by admin-service after an admin acts on a report.
// notification-service emails every distinct reporter.
type ReportResolved struct {
	TargetType  string    `json:"target_type"`  // user | deck
	TargetID    string    `json:"target_id"`
	Action      string    `json:"action"`       // ban_user | hide_deck | delete_deck | dismiss
	Resolution  string    `json:"resolution"`   // banned | deck_hidden | deck_deleted | ""
	ReporterIDs []string  `json:"reporter_ids"`
	ResolvedAt  time.Time `json:"resolved_at"`
}

// CronTick is the payload Cloud Scheduler attaches to the two reminder topics.
// `now` is the UTC instant the scheduler fired (so handler logic is
// reproducible across retries). If absent, the handler uses time.Now().
type CronTick struct {
	Now time.Time `json:"now,omitempty"`
}
