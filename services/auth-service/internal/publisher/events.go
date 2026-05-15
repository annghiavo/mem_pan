package publisher

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

type UserRegisteredEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

type UserUpdatedEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
}

type EmailVerificationRequestedEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PasswordResetRequestedEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"reset_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ReportSubmittedEvent is fired when a user reports another user account.
// admin-service consumes this and persists it to admin_db.reports.
type ReportSubmittedEvent struct {
	ReporterID     string    `json:"reporter_id"`
	TargetType     string    `json:"target_type"`     // always "user" when published by auth-service
	TargetID       string    `json:"target_id"`
	ReasonCategory string    `json:"reason_category"` // inappropriate_content | copyright_violation | spam | harassment | misinformation | other
	Description    string    `json:"description"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, event UserRegisteredEvent) error
	PublishUserUpdated(ctx context.Context, event UserUpdatedEvent) error
	PublishEmailVerificationRequested(ctx context.Context, event EmailVerificationRequestedEvent) error
	PublishPasswordResetRequested(ctx context.Context, event PasswordResetRequestedEvent) error
	PublishReportSubmitted(ctx context.Context, event ReportSubmittedEvent) error
}

type noopPublisher struct{}

func NewNoopPublisher() EventPublisher {
	return &noopPublisher{}
}

func (p *noopPublisher) PublishUserRegistered(_ context.Context, event UserRegisteredEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] user_registered: %s", b)
	return nil
}

func (p *noopPublisher) PublishUserUpdated(_ context.Context, event UserUpdatedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] user_updated: %s", b)
	return nil
}

func (p *noopPublisher) PublishEmailVerificationRequested(_ context.Context, event EmailVerificationRequestedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] email_verification_requested: %s", b)
	return nil
}

func (p *noopPublisher) PublishPasswordResetRequested(_ context.Context, event PasswordResetRequestedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] password_reset_requested: %s", b)
	return nil
}

func (p *noopPublisher) PublishReportSubmitted(_ context.Context, event ReportSubmittedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] report_submitted: %s", b)
	return nil
}
