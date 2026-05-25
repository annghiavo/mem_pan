package subscriber

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/notification-service/internal/authclient"
	"mem_pan/services/notification-service/internal/events"
	"mem_pan/services/notification-service/internal/scheduler"
	"mem_pan/services/notification-service/internal/service"
)

type Handler struct {
	svc        service.NotificationService
	authClient authclient.Client
	sched      *scheduler.Scheduler // nil when statsclient / studyclient are not configured
}

func NewHandler(svc service.NotificationService, authClient authclient.Client, sched *scheduler.Scheduler) *Handler {
	return &Handler{svc: svc, authClient: authClient, sched: sched}
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
	case events.TypeReportResolved:
		return h.handleReportResolved(ctx, data)
	case events.TypeCronStudyReminder:
		return h.handleCronStudyReminder(ctx, data)
	case events.TypeCronStreakWarning:
		return h.handleCronStreakWarning(ctx, data)
	case events.TypeModerationDeckDeleted:
		return h.handleModerationDeckDeleted(ctx, data)
	case events.TypeDeckAppealAvailable:
		return h.handleDeckAppealAvailable(ctx, data)
	case events.TypeAppealDecided:
		return h.handleAppealDecided(ctx, data)
	default:
		log.Printf("[notification] unknown event type %q — skipping", eventType)
		return nil
	}
}

// handleModerationDeckDeleted only sends the FCM push notification.
//
// The deck-owner email is intentionally NOT sent here — admin-service consumes
// the same event, mints a deck_appeals row, and publishes deck.appeal_available
// which carries the appeal token used to build the email's appeal link.
func (h *Handler) handleModerationDeckDeleted(ctx context.Context, data []byte) error {
	var e events.ModerationDeckDeleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if _, err := uuid.Parse(e.UserID); err != nil {
		log.Printf("[moderation] invalid user_id %q — skipping notification", e.UserID)
		return nil
	}

	if pushErr := h.svc.SendModerationDeckDeletedPush(
		ctx, e.UserID, e.DeckID, e.DeckName, e.Reason, len(e.ViolatedCardIDs),
	); pushErr != nil {
		log.Printf("[moderation] fcm push failed user=%s: %v", e.UserID, pushErr)
	}
	return nil
}

// handleDeckAppealAvailable sends the deletion email WITH the appeal link.
func (h *Handler) handleDeckAppealAvailable(ctx context.Context, data []byte) error {
	var e events.DeckAppealAvailable
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if h.authClient == nil {
		log.Printf("[appeal] deck.appeal_available: authClient not configured — skipping email user=%s", e.UserID)
		return nil
	}
	uid, err := uuid.Parse(e.UserID)
	if err != nil {
		log.Printf("[appeal] deck.appeal_available: bad user_id %q — skipping", e.UserID)
		return nil
	}
	user, err := h.authClient.GetUserByID(ctx, uid)
	if err != nil {
		log.Printf("[appeal] deck.appeal_available: auth lookup failed user=%s: %v", e.UserID, err)
		return nil
	}
	if user.Email == "" {
		log.Printf("[appeal] deck.appeal_available: user %s has no email — skipping", e.UserID)
		return nil
	}
	if mailErr := h.svc.SendDeckDeletedWithAppealEmail(
		ctx, e.UserID, user.Email, user.Username, e.DeckName, e.ModerationReason, e.AppealToken,
	); mailErr != nil {
		log.Printf("[appeal] deck.appeal_available: send failed user=%s to=%s: %v", e.UserID, user.Email, mailErr)
	}
	return nil
}

// handleAppealDecided sends the final email — appeal closed, no CTA.
func (h *Handler) handleAppealDecided(ctx context.Context, data []byte) error {
	var e events.AppealDecided
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if h.authClient == nil {
		log.Printf("[appeal] appeal.decided: authClient not configured — skipping email user=%s", e.UserID)
		return nil
	}
	uid, err := uuid.Parse(e.UserID)
	if err != nil {
		log.Printf("[appeal] appeal.decided: bad user_id %q — skipping", e.UserID)
		return nil
	}
	user, err := h.authClient.GetUserByID(ctx, uid)
	if err != nil {
		log.Printf("[appeal] appeal.decided: auth lookup failed user=%s: %v", e.UserID, err)
		return nil
	}
	if user.Email == "" {
		log.Printf("[appeal] appeal.decided: user %s has no email — skipping", e.UserID)
		return nil
	}
	if mailErr := h.svc.SendAppealDecidedEmail(
		ctx, e.UserID, user.Email, user.Username, e.DeckName, e.Decision, e.DecisionNote,
	); mailErr != nil {
		log.Printf("[appeal] appeal.decided: send failed user=%s to=%s: %v", e.UserID, user.Email, mailErr)
	}
	return nil
}

func (h *Handler) handleCronStudyReminder(ctx context.Context, data []byte) error {
	if h.sched == nil {
		log.Printf("[cron] study_reminder fired but scheduler not configured — skipping")
		return nil
	}
	now := decodeTickTime(data)
	if err := h.sched.HandleStudyReminderTick(ctx, now); err != nil {
		log.Printf("[cron] study_reminder tick failed: %v (acking — next tick fires in 15m)", err)
	}
	return nil
}

func (h *Handler) handleCronStreakWarning(ctx context.Context, data []byte) error {
	if h.sched == nil {
		log.Printf("[cron] streak_warning fired but scheduler not configured — skipping")
		return nil
	}
	now := decodeTickTime(data)
	if err := h.sched.HandleStreakWarningTick(ctx, now); err != nil {
		log.Printf("[cron] streak_warning tick failed: %v (acking — next tick fires in 15m)", err)
	}
	return nil
}

func decodeTickTime(data []byte) time.Time {
	var t events.CronTick
	if err := json.Unmarshal(data, &t); err != nil || t.Now.IsZero() {
		return time.Now().UTC()
	}
	return t.Now.UTC()
}

// Email send failures (SMTP rate-limit, recipient bounce, transient DNS, etc.)
// are NOT retried via Pub/Sub. Retrying through Pub/Sub re-hits the same SMTP
// relay within seconds and keeps Gmail's rate-limit alive indefinitely. Log
// the failure and ack the message; the user can re-trigger if needed.
func (h *Handler) handleUserRegistered(ctx context.Context, data []byte) error {
	var e events.UserRegistered
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if err := h.svc.SendWelcomeEmail(ctx, e.UserID, e.Email, e.Username); err != nil {
		log.Printf("[notification] welcome email to %s failed: %v (acked)", e.Email, err)
	}
	return nil
}

func (h *Handler) handleEmailVerification(ctx context.Context, data []byte) error {
	var e events.EmailVerificationRequested
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if err := h.svc.SendVerificationEmail(ctx, e.UserID, e.Email, e.UserID, e.Token); err != nil {
		log.Printf("[notification] verification email to %s failed: %v (acked)", e.Email, err)
	}
	return nil
}

func (h *Handler) handlePasswordReset(ctx context.Context, data []byte) error {
	var e events.PasswordResetRequested
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if err := h.svc.SendPasswordResetEmail(ctx, e.UserID, e.Email, e.UserID, e.Token); err != nil {
		log.Printf("[notification] password reset email to %s failed: %v (acked)", e.Email, err)
	}
	return nil
}

func (h *Handler) handleDeckCloneCompleted(ctx context.Context, data []byte) error {
	var e events.DeckCloneCompleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if err := h.svc.SendDeckCloneReadyPush(ctx, e.UserID, e.DeckID, e.DeckName, e.CardCount); err != nil {
		log.Printf("[notification] deck clone push user=%s deck=%s failed: %v (acked)", e.UserID, e.DeckID, err)
	}
	return nil
}

// handleReportResolved fans out one email per distinct reporter, looking up
// the reporter's email + username via auth-service. Failures for a single
// reporter are logged but do not block the rest.
func (h *Handler) handleReportResolved(ctx context.Context, data []byte) error {
	var e events.ReportResolved
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if h.authClient == nil {
		log.Printf("[notification] report.resolved: no authClient configured, skipping")
		return nil
	}

	outcome := outcomeMessage(e.TargetType, e.Action)
	for _, idStr := range e.ReporterIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			log.Printf("[notification] report.resolved: bad reporter_id %q: %v", idStr, err)
			continue
		}
		user, err := h.authClient.GetUserByID(ctx, id)
		if err != nil {
			log.Printf("[notification] report.resolved: lookup %s failed: %v", idStr, err)
			continue
		}
		if err := h.svc.SendReportResolvedEmail(ctx, idStr, user.Email, user.Username, outcome); err != nil {
			log.Printf("[notification] report.resolved: send to %s failed: %v", user.Email, err)
		}
	}

	// The deck-owner email is intentionally NOT sent here. For `delete_deck`,
	// admin-service mints a deck-appeal row and publishes deck.appeal_available
	// — that's the path that emails the owner with the appeal link. For
	// `hide_deck`, the owner is not emailed (the deck simply hides).

	return nil
}

func outcomeMessage(targetType, action string) string {
	switch action {
	case "ban_user":
		return "The reported user has been banned."
	case "hide_deck":
		return "The reported deck has been hidden from public view."
	case "delete_deck":
		return "The reported deck has been removed."
	case "dismiss":
		if targetType == "user" {
			return "After review, the reported user does not violate our policies. No action was taken."
		}
		return "After review, the reported content does not violate our policies. No action was taken."
	default:
		return "Your report has been reviewed."
	}
}
