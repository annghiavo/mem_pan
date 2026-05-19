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
	default:
		log.Printf("[notification] unknown event type %q — skipping", eventType)
		return nil
	}
}

func (h *Handler) handleCronStudyReminder(ctx context.Context, data []byte) error {
	if h.sched == nil {
		log.Printf("[cron] study_reminder fired but scheduler not configured — skipping")
		return nil
	}
	now := decodeTickTime(data)
	return h.sched.HandleStudyReminderTick(ctx, now)
}

func (h *Handler) handleCronStreakWarning(ctx context.Context, data []byte) error {
	if h.sched == nil {
		log.Printf("[cron] streak_warning fired but scheduler not configured — skipping")
		return nil
	}
	now := decodeTickTime(data)
	return h.sched.HandleStreakWarningTick(ctx, now)
}

func decodeTickTime(data []byte) time.Time {
	var t events.CronTick
	if err := json.Unmarshal(data, &t); err != nil || t.Now.IsZero() {
		return time.Now().UTC()
	}
	return t.Now.UTC()
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
