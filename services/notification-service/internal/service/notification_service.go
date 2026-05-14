package service

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"mem_pan/services/notification-service/internal/db"
	"mem_pan/services/notification-service/internal/fcm"
	"mem_pan/services/notification-service/internal/mailer"
	"mem_pan/services/notification-service/internal/repository"
)

type NotificationService interface {
	RegisterDeviceToken(ctx context.Context, userID uuid.UUID, token, deviceName string) error
	UnregisterDeviceToken(ctx context.Context, userID uuid.UUID, token string) error

	SendWelcomeEmail(ctx context.Context, userID, email, username string) error
	SendVerificationEmail(ctx context.Context, userID, email, username, token string) error
	SendPasswordResetEmail(ctx context.Context, userID, email, username, token string) error
	SendDeckCloneReadyPush(ctx context.Context, userID, deckID, deckName string, cardCount int32) error
}

type Config struct {
	AppBaseURL string // e.g. "https://mempan.app"
}

type service struct {
	repo   repository.NotificationRepository
	mailer mailer.Mailer
	fcm    fcm.Sender
	cfg    Config
}

func New(repo repository.NotificationRepository, m mailer.Mailer, f fcm.Sender, cfg Config) NotificationService {
	return &service{repo: repo, mailer: m, fcm: f, cfg: cfg}
}

func (s *service) RegisterDeviceToken(ctx context.Context, userID uuid.UUID, token, deviceName string) error {
	_, err := s.repo.UpsertFCMToken(ctx, userID, token, deviceName)
	return err
}

func (s *service) UnregisterDeviceToken(ctx context.Context, userID uuid.UUID, token string) error {
	return s.repo.DeleteFCMToken(ctx, userID, token)
}

func (s *service) SendWelcomeEmail(ctx context.Context, userID, email, username string) error {
	err := s.mailer.SendWelcome(email, username)
	s.log(ctx, userID, "welcome", "email", email, err)
	return err
}

func (s *service) SendVerificationEmail(ctx context.Context, userID, email, username, token string) error {
	url := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.AppBaseURL, token)
	err := s.mailer.SendEmailVerification(email, username, url)
	s.log(ctx, userID, "email_verification", "email", email, err)
	return err
}

func (s *service) SendPasswordResetEmail(ctx context.Context, userID, email, username, token string) error {
	url := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.AppBaseURL, token)
	err := s.mailer.SendPasswordReset(email, username, url)
	s.log(ctx, userID, "password_reset", "email", email, err)
	return err
}

func (s *service) SendDeckCloneReadyPush(ctx context.Context, userID, deckID, deckName string, cardCount int32) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user_id: %w", err)
	}

	tokens, err := s.repo.ListFCMTokensByUser(ctx, uid)
	if err != nil {
		return fmt.Errorf("list fcm tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	rawTokens := make([]string, len(tokens))
	for i, t := range tokens {
		rawTokens[i] = t.Token
	}

	title := "Deck Clone Ready"
	body := fmt.Sprintf("Your copy of \"%s\" (%d cards) is ready to study!", deckName, cardCount)
	data := map[string]string{
		"type":    "deck_clone_completed",
		"deck_id": deckID,
	}

	err = s.fcm.Send(ctx, rawTokens, title, body, data)
	s.log(ctx, userID, "deck_clone_ready", "fcm", userID, err)
	return err
}

func (s *service) log(ctx context.Context, userID, notifType, channel, recipient string, sendErr error) {
	uid, parseErr := uuid.Parse(userID)

	status := "sent"
	var errMsg *string
	if sendErr != nil {
		status = "failed"
		msg := sendErr.Error()
		errMsg = &msg
		log.Printf("[notification] failed to send %s via %s to %s: %v", notifType, channel, recipient, sendErr)
	}

	var uidPtr *uuid.UUID
	if parseErr == nil {
		uidPtr = &uid
	}

	if logErr := s.repo.LogNotification(ctx, db.LogNotificationParams{
		UserID:           uidPtr,
		NotificationType: notifType,
		Channel:          channel,
		Recipient:        recipient,
		Status:           status,
		ErrorMessage:     errMsg,
	}); logErr != nil {
		log.Printf("[notification] failed to write log: %v", logErr)
	}
}
