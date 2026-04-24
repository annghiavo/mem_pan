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
	CreatedAt time.Time `json:"created_at"`
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

type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, event UserRegisteredEvent) error
	PublishEmailVerificationRequested(ctx context.Context, event EmailVerificationRequestedEvent) error
	PublishPasswordResetRequested(ctx context.Context, event PasswordResetRequestedEvent) error
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
