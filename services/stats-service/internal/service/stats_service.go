package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/stats-service/internal/db"
	"mem_pan/services/stats-service/internal/repository"
)

type ReminderState struct {
	UserID             uuid.UUID
	CurrentStreak      int32
	LastStudiedDate    *time.Time // nil if user has never studied
	OptimalHourWeekday *int16     // nil when not yet computed
	OptimalHourWeekend *int16
	ReminderLocalTime  string // "HH:MM:SS"
}

type StatsService interface {
	GetUserStats(ctx context.Context, userID uuid.UUID) (db.UserStat, error)
	GetHeatmap(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]db.DailyStat, error)
	GetDeckStats(ctx context.Context, deckID uuid.UUID) (db.DeckStat, error)
	ListDeckStatsByUser(ctx context.Context, userID uuid.UUID) ([]db.DeckStat, error)
	GetDeckProgress(ctx context.Context, deckID uuid.UUID, from, to time.Time) ([]db.DeckProgressSnapshot, error)

	// ListReminderState is called by notification-service from the cron
	// handlers. If onlyActiveStreak is true, only users with current_streak>=1
	// are returned.
	ListReminderState(ctx context.Context, onlyActiveStreak bool) ([]ReminderState, error)

	// BumpActivityBucket records a single review in the (hour, day_type)
	// histogram. Called by the study-event subscriber.
	BumpActivityBucket(ctx context.Context, userID uuid.UUID, hour int16, dayType int16, count int32) error

	// RecomputeOptimalHours scans the user's histogram and writes the
	// argmax-hour for weekday + weekend buckets to user_stats. Called by a
	// nightly cron (not part of this change set — see TODO).
	RecomputeOptimalHours(ctx context.Context, userID uuid.UUID) error
}

type statsService struct {
	repo repository.StatsRepository
}

func New(repo repository.StatsRepository) StatsService {
	return &statsService{repo: repo}
}

func (s *statsService) GetUserStats(ctx context.Context, userID uuid.UUID) (db.UserStat, error) {
	return s.repo.GetUserStats(ctx, userID)
}

func (s *statsService) GetHeatmap(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]db.DailyStat, error) {
	return s.repo.ListDailyStats(ctx, userID, from, to)
}

func (s *statsService) GetDeckStats(ctx context.Context, deckID uuid.UUID) (db.DeckStat, error) {
	return s.repo.GetDeckStats(ctx, deckID)
}

func (s *statsService) ListDeckStatsByUser(ctx context.Context, userID uuid.UUID) ([]db.DeckStat, error) {
	return s.repo.ListDeckStatsByUser(ctx, userID)
}

func (s *statsService) GetDeckProgress(ctx context.Context, deckID uuid.UUID, from, to time.Time) ([]db.DeckProgressSnapshot, error) {
	return s.repo.ListDeckProgressSnapshots(ctx, deckID, from, to)
}

func (s *statsService) ListReminderState(ctx context.Context, onlyActiveStreak bool) ([]ReminderState, error) {
	rows, err := s.repo.ListReminderState(ctx, onlyActiveStreak)
	if err != nil {
		return nil, err
	}
	out := make([]ReminderState, 0, len(rows))
	for _, r := range rows {
		out = append(out, ReminderState{
			UserID:             r.UserID,
			CurrentStreak:      r.CurrentStreak,
			LastStudiedDate:    r.LastStudiedDate,
			OptimalHourWeekday: r.OptimalHourWeekday,
			OptimalHourWeekend: r.OptimalHourWeekend,
			ReminderLocalTime:  r.ReminderLocalTime,
		})
	}
	return out, nil
}

func (s *statsService) BumpActivityBucket(ctx context.Context, userID uuid.UUID, hour int16, dayType int16, count int32) error {
	return s.repo.BumpActivityBucket(ctx, userID, hour, dayType, count)
}

// RecomputeOptimalHours computes argmax over the user's activity histogram,
// splitting weekday / weekend buckets. If a bucket has fewer than minSamples
// reviews total, it stays NULL so the cron falls back to the default time.
func (s *statsService) RecomputeOptimalHours(ctx context.Context, userID uuid.UUID) error {
	const minSamples = 5

	buckets, err := s.repo.ListActivityBuckets(ctx, userID)
	if err != nil {
		return err
	}

	// Sum + argmax per day_type.
	var sum [2]int32
	var bestHour [2]int16 = [2]int16{-1, -1}
	var bestCount [2]int32

	for _, b := range buckets {
		dt := int(b.DayType)
		if dt < 0 || dt > 1 {
			continue
		}
		sum[dt] += b.ReviewCount
		if b.ReviewCount > bestCount[dt] {
			bestCount[dt] = b.ReviewCount
			bestHour[dt] = b.HourOfDay
		}
	}

	var wkday, wkend *int16
	if sum[0] >= minSamples {
		h := bestHour[0]
		wkday = &h
	}
	if sum[1] >= minSamples {
		h := bestHour[1]
		wkend = &h
	}

	return s.repo.SetOptimalHours(ctx, userID, wkday, wkend)
}
