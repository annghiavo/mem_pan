// Package scheduler implements the two reminder cron handlers that
// notification-service exposes as Pub/Sub push endpoints:
//
//   1. study_reminder  — fired every 15 min by Cloud Scheduler. For each user,
//      it computes send_time = optimal_hour(day_type) − 15 minutes in the
//      user's IANA timezone, and pushes "you have N cards to review" when the
//      cron tick lands inside the 15-minute window ending at send_time.
//
//   2. streak_warning — fired every 15 min by Cloud Scheduler. For each user
//      whose current_streak >= 1, it pushes a warning when the cron tick lands
//      inside the 15-minute window ending at the user's reminder_local_time
//      (default 21:00) AND they have not yet studied today (last_studied_date
//      != today-in-tz). Streaks >= 3 include the streak number; otherwise the
//      copy is generic.
//
// Both handlers fan out FCM in parallel (bounded concurrency). FCM send
// failures for individual users are logged but do not fail the cron tick —
// Cloud Scheduler will retry the topic on next tick.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/notification-service/internal/authclient"
	"mem_pan/services/notification-service/internal/fcm"
	"mem_pan/services/notification-service/internal/repository"
	"mem_pan/services/notification-service/internal/statsclient"
	"mem_pan/services/notification-service/internal/studyclient"
)

// TickWindow is the half-open interval [send_time - TickWindow, send_time)
// during which a tick is allowed to fire a notification. It must be >= the
// Cloud Scheduler frequency. The scheduler fires every 15 min, so 15m matches.
const TickWindow = 15 * time.Minute

// MaxFanOut bounds the number of concurrent per-user goroutines per tick.
const MaxFanOut = 32

// Scheduler holds dependencies for both cron handlers.
type Scheduler struct {
	repo   repository.NotificationRepository
	fcm    fcm.Sender
	auth   authclient.Client
	stats  statsclient.Client
	study  studyclient.Client
}

func New(
	repo repository.NotificationRepository,
	f fcm.Sender,
	auth authclient.Client,
	stats statsclient.Client,
	study studyclient.Client,
) *Scheduler {
	return &Scheduler{repo: repo, fcm: f, auth: auth, stats: stats, study: study}
}

// HandleStudyReminderTick is the entry point for the `cron.study_reminder`
// Pub/Sub event. `now` is the UTC instant of the tick.
func (s *Scheduler) HandleStudyReminderTick(ctx context.Context, now time.Time) error {
	users, err := s.stats.ListReminderState(ctx, false)
	if err != nil {
		return fmt.Errorf("list reminder state: %w", err)
	}

	sem := make(chan struct{}, MaxFanOut)
	var wg sync.WaitGroup
	for _, u := range users {
		u := u
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.runStudyReminderForUser(ctx, u, now)
		}()
	}
	wg.Wait()
	return nil
}

// HandleStreakWarningTick is the entry point for the `cron.streak_warning`
// Pub/Sub event.
func (s *Scheduler) HandleStreakWarningTick(ctx context.Context, now time.Time) error {
	users, err := s.stats.ListReminderState(ctx, true) // only_active_streak
	if err != nil {
		return fmt.Errorf("list reminder state (streak): %w", err)
	}

	sem := make(chan struct{}, MaxFanOut)
	var wg sync.WaitGroup
	for _, u := range users {
		u := u
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.runStreakWarningForUser(ctx, u, now)
		}()
	}
	wg.Wait()
	return nil
}

// ── Per-user logic ──────────────────────────────────────────────────────────

func (s *Scheduler) runStudyReminderForUser(ctx context.Context, st statsclient.ReminderState, now time.Time) {
	uid, err := uuid.Parse(st.UserID)
	if err != nil {
		return
	}
	user, err := s.auth.GetUserByID(ctx, uid)
	if err != nil {
		log.Printf("[cron-study] auth lookup %s: %v", st.UserID, err)
		return
	}
	loc := loadLoc(user.Timezone)
	localNow := now.In(loc)

	// optimal_hour: pick weekday vs weekend bucket. If unset (-1), fall back
	// to 1 hour before the user's reminder_local_time (default 21:00 → 20:00).
	optimalHour := pickOptimalHour(st, localNow)
	if optimalHour < 0 {
		warnH, _ := parseHM(st.ReminderLocalTime)
		optimalHour = warnH - 1
		if optimalHour < 0 {
			optimalHour = 20
		}
	}

	// send_time = optimal_hour - 15 minutes, anchored to today in user's tz.
	sendTime := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(),
		optimalHour, 0, 0, 0, loc,
	).Add(-15 * time.Minute)

	if !inWindow(localNow, sendTime, TickWindow) {
		return
	}

	due, err := s.study.CountDueForUser(ctx, st.UserID, user.Timezone)
	if err != nil {
		log.Printf("[cron-study] due count %s: %v", st.UserID, err)
		return
	}
	if due <= 0 {
		return
	}

	if dedupeSkip(s.repo, ctx, uid, "study_reminder", localNow) {
		return
	}

	title := "Time to study"
	body := studyReminderBody(due, st.CurrentStreak)
	data := map[string]string{
		"type":      "study_reminder",
		"due_count": fmt.Sprintf("%d", due),
		"streak":    fmt.Sprintf("%d", st.CurrentStreak),
	}
	s.dispatch(ctx, uid, "study_reminder", title, body, data)
}

func (s *Scheduler) runStreakWarningForUser(ctx context.Context, st statsclient.ReminderState, now time.Time) {
	uid, err := uuid.Parse(st.UserID)
	if err != nil {
		return
	}
	user, err := s.auth.GetUserByID(ctx, uid)
	if err != nil {
		log.Printf("[cron-streak] auth lookup %s: %v", st.UserID, err)
		return
	}
	loc := loadLoc(user.Timezone)
	localNow := now.In(loc)
	todayLocal := localNow.Format("2006-01-02")

	// Already studied today? Then the streak is safe — skip.
	if st.LastStudiedDate == todayLocal {
		return
	}

	warnH, warnM := parseHM(st.ReminderLocalTime)
	sendTime := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(),
		warnH, warnM, 0, 0, loc,
	)

	if !inWindow(localNow, sendTime, TickWindow) {
		return
	}

	if dedupeSkip(s.repo, ctx, uid, "streak_warning", localNow) {
		return
	}

	due, err := s.study.CountDueForUser(ctx, st.UserID, user.Timezone)
	if err != nil {
		// Still send the streak warning even if we can't get the count.
		log.Printf("[cron-streak] due count %s: %v", st.UserID, err)
		due = 0
	}

	title := "Don't lose your streak"
	body := streakWarningBody(due, st.CurrentStreak)
	data := map[string]string{
		"type":      "streak_warning",
		"due_count": fmt.Sprintf("%d", due),
		"streak":    fmt.Sprintf("%d", st.CurrentStreak),
	}
	s.dispatch(ctx, uid, "streak_warning", title, body, data)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func loadLoc(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// pickOptimalHour returns optimal_hour_weekday on Mon-Fri and
// optimal_hour_weekend on Sat-Sun. Returns -1 if the chosen bucket is unset.
func pickOptimalHour(st statsclient.ReminderState, localNow time.Time) int {
	wd := localNow.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return int(st.OptimalHourWeekend)
	}
	return int(st.OptimalHourWeekday)
}

// inWindow reports whether `t` lies in (sendTime - window, sendTime].
// This ensures a given send_time is hit by exactly one 15-minute tick.
func inWindow(t, sendTime time.Time, window time.Duration) bool {
	if t.After(sendTime) {
		return false
	}
	return !t.Before(sendTime.Add(-window))
}

// parseHM parses "HH:MM" or "HH:MM:SS". Defaults to 21:00 on any error.
func parseHM(s string) (int, int) {
	if s == "" {
		return 21, 0
	}
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n < 2 || h < 0 || h > 23 || m < 0 || m > 59 {
		return 21, 0
	}
	return h, m
}

// studyReminderBody renders the push body.
// The notification always shows the due-card count. Streak text is appended
// only when streak >= 3 (mirroring the streak_warning copy).
func studyReminderBody(due, streak int32) string {
	if streak >= 3 {
		return fmt.Sprintf("You have %d card%s to review — keep your %d-day streak alive!",
			due, plural(due), streak)
	}
	return fmt.Sprintf("You have %d card%s to review today.", due, plural(due))
}

// streakWarningBody mirrors the user's spec:
//
//   streak >= 3 → "X cards to review — about to lose your N-day streak!"
//   streak <  3 → "X cards to review — about to lose your streak!"
//
// If `due` is 0 (count call failed), the count phrase is omitted.
func streakWarningBody(due, streak int32) string {
	prefix := ""
	if due > 0 {
		prefix = fmt.Sprintf("%d card%s to review — ", due, plural(due))
	}
	if streak >= 3 {
		return fmt.Sprintf("%sabout to lose your %d-day streak!", prefix, streak)
	}
	return prefix + "about to lose your streak!"
}

func plural(n int32) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// dispatch sends to all of the user's FCM tokens and logs the result.
func (s *Scheduler) dispatch(ctx context.Context, uid uuid.UUID, notifType, title, body string, data map[string]string) {
	tokens, err := s.repo.ListFCMTokensByUser(ctx, uid)
	if err != nil {
		log.Printf("[cron] %s: list tokens %s: %v", notifType, uid, err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	raw := make([]string, len(tokens))
	for i, t := range tokens {
		raw[i] = t.Token
	}
	if err := s.fcm.Send(ctx, raw, title, body, data); err != nil {
		log.Printf("[cron] %s: fcm send %s: %v", notifType, uid, err)
		return
	}
	log.Printf("[cron] %s: sent to %s (%d devices)", notifType, uid, len(raw))
}

// dedupeSkip checks whether we already sent this notification type to this
// user today (in their local timezone). Returns true if the caller should skip.
// Uses the notification_log table which is already part of the schema.
func dedupeSkip(repo repository.NotificationRepository, ctx context.Context, uid uuid.UUID, notifType string, localNow time.Time) bool {
	since := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location()).UTC()
	count, err := repo.CountRecentNotifications(ctx, uid, notifType, since)
	if err != nil {
		// On lookup failure we err on the side of sending. The push is
		// idempotent enough — worst case the user gets a duplicate once.
		log.Printf("[cron] dedupe lookup %s/%s: %v", uid, notifType, err)
		return false
	}
	return count > 0
}
