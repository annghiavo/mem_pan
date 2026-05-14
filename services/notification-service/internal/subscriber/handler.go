package subscriber

import (
	"context"
	"encoding/json"
	"log"

	"mem_pan/services/notification-service/internal/events"
	"mem_pan/services/notification-service/internal/service"
)

type Handler struct {
	svc service.NotificationService
}

func NewHandler(svc service.NotificationService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Dispatch(ctx context.Context, eventType string, data []byte) error {
	switch eventType {
	case events.TypeUserRegistered:
		return h.handleUserRegistered(ctx, data)
	case events.TypeEmailVerificationRequested:
		return h.handleEmailVerification(ctx, data)
	case events.TypePasswordResetRequested:
		return h.handlePasswordReset(ctx, data)
	case events.TypeDeckCloneCompleted:
		return h.handleDeckCloneCompleted(ctx, data)
	default:
		log.Printf("[notification] unknown event type %q — skipping", eventType)
		return nil
	}
}

func (h *Handler) handleUserRegistered(ctx context.Context, data []byte) error {
	var e events.UserRegistered
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.SendWelcomeEmail(ctx, e.UserID, e.Email, e.Username)
}

func (h *Handler) handleEmailVerification(ctx context.Context, data []byte) error {
	var e events.EmailVerificationRequested
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.SendVerificationEmail(ctx, e.UserID, e.Email, e.UserID, e.Token)
}

func (h *Handler) handlePasswordReset(ctx context.Context, data []byte) error {
	var e events.PasswordResetRequested
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.SendPasswordResetEmail(ctx, e.UserID, e.Email, e.UserID, e.Token)
}

func (h *Handler) handleDeckCloneCompleted(ctx context.Context, data []byte) error {
	var e events.DeckCloneCompleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.SendDeckCloneReadyPush(ctx, e.UserID, e.DeckID, e.DeckName, e.CardCount)
}
