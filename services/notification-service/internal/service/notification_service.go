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
	"mem_pan/services/notification-service/internal/scheduler"
)

// templateDataFromMap converts the gRPC string-map payload into the shape
// expected by html/text templates (which dispatch on field/key names).
func templateDataFromMap(data map[string]string) map[string]string {
	if data == nil {
		return map[string]string{}
	}
	return data
}

type NotificationService interface {
	RegisterDeviceToken(ctx context.Context, userID uuid.UUID, token, deviceName string) error
	UnregisterDeviceToken(ctx context.Context, userID uuid.UUID, token string) error
	SendTestNotification(ctx context.Context, userID uuid.UUID, p TestNotificationParams) (TestNotificationResult, error)

	SendWelcomeEmail(ctx context.Context, userID, email, username string) error
	SendVerificationEmail(ctx context.Context, userID, email, username, token string) error
	SendPasswordResetEmail(ctx context.Context, userID, email, username, token string) error
	SendDeckCloneReadyPush(ctx context.Context, userID, deckID, deckName string, cardCount int32) error
	SendReportResolvedEmail(ctx context.Context, userID, email, username, outcome string) error
	SendDeckModerationEmail(ctx context.Context, userID, email, username, deckStatus string) error
	SendModerationDeckDeletedPush(ctx context.Context, userID, deckID, reason string, violatedCardCount int) error

	// Email template administration.
	ListEmailTemplates(ctx context.Context) ([]db.EmailTemplate, error)
	GetEmailTemplate(ctx context.Context, key, locale string) (db.EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, p UpdateTemplateParams) (db.EmailTemplate, error)
	PreviewEmailTemplate(ctx context.Context, key, locale string, data map[string]string) (RenderedEmail, error)
	SendTestEmail(ctx context.Context, key, locale, to string, data map[string]string) error
}

type UpdateTemplateParams struct {
	Key       string
	Locale    string
	Subject   string
	HTMLBody  string
	TextBody  string
	UpdatedBy uuid.UUID
}

type RenderedEmail struct {
	Subject  string
	HTMLBody string
	TextBody string
}

type TestNotificationParams struct {
	Type     string // "study_reminder" (default) or "streak_warning"
	Token    string // optional; if set, push goes only to this token
	DueCount int32
	Streak   int32
}

type TestNotificationResult struct {
	DeviceCount int32
	Title       string
	Body        string
}

type Config struct {
	AppBaseURL string // e.g. "https://mempan.app"
}

type service struct {
	repo   repository.NotificationRepository
	mailer mailer.Mailer
	fcm    fcm.Sender
	cache  *mailer.CachedStore // nil-safe; when nil, template updates skip cache invalidation
	cfg    Config
}

func New(repo repository.NotificationRepository, m mailer.Mailer, f fcm.Sender, cache *mailer.CachedStore, cfg Config) NotificationService {
	return &service{repo: repo, mailer: m, fcm: f, cache: cache, cfg: cfg}
}

func (s *service) RegisterDeviceToken(ctx context.Context, userID uuid.UUID, token, deviceName string) error {
	_, err := s.repo.UpsertFCMToken(ctx, userID, token, deviceName)
	return err
}

func (s *service) UnregisterDeviceToken(ctx context.Context, userID uuid.UUID, token string) error {
	return s.repo.DeleteFCMToken(ctx, userID, token)
}

// SendTestNotification fires a push that mirrors the production payload from
// the reminder cron (same title/body builder, same data fields). It exists so
// the Android client (or a curl/grpcurl tool) can verify FCM end-to-end
// without waiting for the cron tick window, due cards, or dedup state.
func (s *service) SendTestNotification(ctx context.Context, userID uuid.UUID, p TestNotificationParams) (TestNotificationResult, error) {
	notifType := p.Type
	if notifType == "" {
		notifType = "study_reminder"
	}

	var title, body string
	switch notifType {
	case "study_reminder":
		title = "Time to study"
		body = scheduler.StudyReminderBody(p.DueCount, p.Streak)
	case "streak_warning":
		title = "Don't lose your streak"
		body = scheduler.StreakWarningBody(p.DueCount, p.Streak)
	default:
		return TestNotificationResult{}, fmt.Errorf("unsupported notification_type %q (expected study_reminder or streak_warning)", notifType)
	}

	var tokens []string
	if p.Token != "" {
		tokens = []string{p.Token}
	} else {
		rows, err := s.repo.ListFCMTokensByUser(ctx, userID)
		if err != nil {
			return TestNotificationResult{}, fmt.Errorf("list fcm tokens: %w", err)
		}
		tokens = make([]string, len(rows))
		for i, t := range rows {
			tokens[i] = t.Token
		}
	}

	data := map[string]string{
		"type":      notifType,
		"due_count": fmt.Sprintf("%d", p.DueCount),
		"streak":    fmt.Sprintf("%d", p.Streak),
		"test":      "true",
	}

	result := TestNotificationResult{
		DeviceCount: int32(len(tokens)),
		Title:       title,
		Body:        body,
	}

	if len(tokens) == 0 {
		return result, nil
	}

	err := s.fcm.Send(ctx, tokens, title, body, data)
	s.log(ctx, userID.String(), "test_"+notifType, "fcm", userID.String(), err)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *service) SendWelcomeEmail(ctx context.Context, userID, email, username string) error {
	err := s.mailer.SendWelcome(ctx, email, username)
	s.log(ctx, userID, "welcome", "email", email, err)
	return err
}

func (s *service) SendVerificationEmail(ctx context.Context, userID, email, username, token string) error {
	url := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.AppBaseURL, token)
	err := s.mailer.SendEmailVerification(ctx, email, username, url)
	s.log(ctx, userID, "email_verification", "email", email, err)
	return err
}

func (s *service) SendPasswordResetEmail(ctx context.Context, userID, email, username, token string) error {
	url := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.AppBaseURL, token)
	err := s.mailer.SendPasswordReset(ctx, email, username, url)
	s.log(ctx, userID, "password_reset", "email", email, err)
	return err
}

func (s *service) SendReportResolvedEmail(ctx context.Context, userID, email, username, outcome string) error {
	err := s.mailer.SendReportResolved(ctx, email, username, outcome)
	s.log(ctx, userID, "report_resolved", "email", email, err)
	return err
}

func (s *service) SendDeckModerationEmail(ctx context.Context, userID, email, username, deckStatus string) error {
	err := s.mailer.SendDeckModeration(ctx, email, username, deckStatus)
	s.log(ctx, userID, "deck_moderation", "email", email, err)
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

func (s *service) SendModerationDeckDeletedPush(
	ctx context.Context, userID, deckID, reason string, violatedCardCount int,
) error {
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

	title := "Deck Removed by Content Moderation"
	body := fmt.Sprintf(
		"Your deck was removed because %d card(s) violated our content policy (%s). "+
			"You can appeal this decision from your account settings.",
		violatedCardCount, reason,
	)
	data := map[string]string{
		"type":    "moderation_deck_deleted",
		"deck_id": deckID,
		"reason":  reason,
	}

	err = s.fcm.Send(ctx, rawTokens, title, body, data)
	s.log(ctx, userID, "moderation_deck_deleted", "fcm", userID, err)
	return err
}

// ---------- Email template administration ----------

func (s *service) ListEmailTemplates(ctx context.Context) ([]db.EmailTemplate, error) {
	return s.repo.ListEmailTemplates(ctx)
}

func (s *service) GetEmailTemplate(ctx context.Context, key, locale string) (db.EmailTemplate, error) {
	if locale == "" {
		locale = mailer.DefaultLocale
	}
	return s.repo.GetEmailTemplate(ctx, key, locale)
}

func (s *service) UpdateEmailTemplate(ctx context.Context, p UpdateTemplateParams) (db.EmailTemplate, error) {
	if p.Locale == "" {
		p.Locale = mailer.DefaultLocale
	}
	tpl := mailer.Template{Subject: p.Subject, HTML: p.HTMLBody, Text: p.TextBody}
	if err := mailer.ValidateTemplate(tpl); err != nil {
		return db.EmailTemplate{}, fmt.Errorf("invalid template: %w", err)
	}

	updatedBy := p.UpdatedBy
	updated, err := s.repo.UpdateEmailTemplate(ctx, db.UpdateEmailTemplateParams{
		TemplateKey: p.Key,
		Locale:      p.Locale,
		Subject:     p.Subject,
		HtmlBody:    p.HTMLBody,
		TextBody:    p.TextBody,
		UpdatedBy:   &updatedBy,
	})
	if err != nil {
		return db.EmailTemplate{}, err
	}

	if err := s.repo.InsertEmailTemplateVersion(ctx, db.InsertEmailTemplateVersionParams{
		TemplateID: updated.ID,
		Version:    updated.Version,
		Subject:    updated.Subject,
		HtmlBody:   updated.HtmlBody,
		TextBody:   updated.TextBody,
		UpdatedBy:  &updatedBy,
	}); err != nil {
		log.Printf("[notification] failed to write template version snapshot: %v", err)
	}

	if s.cache != nil {
		s.cache.Invalidate(p.Key, p.Locale)
	}
	return updated, nil
}

func (s *service) PreviewEmailTemplate(ctx context.Context, key, locale string, data map[string]string) (RenderedEmail, error) {
	if locale == "" {
		locale = mailer.DefaultLocale
	}
	row, err := s.repo.GetEmailTemplate(ctx, key, locale)
	if err != nil {
		return RenderedEmail{}, err
	}
	subject, html, text, err := mailer.Render(mailer.Template{
		Subject: row.Subject, HTML: row.HtmlBody, Text: row.TextBody,
	}, templateDataFromMap(data))
	if err != nil {
		return RenderedEmail{}, err
	}
	return RenderedEmail{Subject: subject, HTMLBody: html, TextBody: text}, nil
}

func (s *service) SendTestEmail(ctx context.Context, key, locale, to string, data map[string]string) error {
	rendered, err := s.PreviewEmailTemplate(ctx, key, locale, data)
	if err != nil {
		return err
	}
	if err := s.mailer.SendRaw(ctx, to, rendered.Subject, rendered.HTMLBody, rendered.TextBody); err != nil {
		s.log(ctx, "", "test_"+key, "email", to, err)
		return err
	}
	s.log(ctx, "", "test_"+key, "email", to, nil)
	return nil
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
	} else {
		log.Printf("[notification] sent %s via %s to %s", notifType, channel, recipient)
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
