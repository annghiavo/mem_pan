package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/stats-service/internal/db"
	"mem_pan/services/stats-service/internal/domain"
)

type StatsRepository interface {
	// user_stats
	CreateUserStats(ctx context.Context, userID uuid.UUID, username, avatarURL string) (db.UserStat, error)
	GetUserStats(ctx context.Context, userID uuid.UUID) (db.UserStat, error)
	IncrementReview(ctx context.Context, arg db.IncrementReviewParams) error
	UpdateStreak(ctx context.Context, userID uuid.UUID, streak int32, lastDate time.Time) error
	IncrementUserCards(ctx context.Context, userID uuid.UUID) error
	DecrementUserCards(ctx context.Context, userID uuid.UUID, count int32) error
	UpdateUserProfile(ctx context.Context, userID uuid.UUID, username, avatarURL string) error

	// daily_stats
	UpsertDailyStats(ctx context.Context, arg db.UpsertDailyStatsParams) error
	ListDailyStats(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]db.DailyStat, error)

	// deck_stats
	CreateDeckStats(ctx context.Context, deckID, userID uuid.UUID, deckName string) error
	GetDeckStats(ctx context.Context, deckID uuid.UUID) (db.DeckStat, error)
	ListDeckStatsByUser(ctx context.Context, userID uuid.UUID) ([]db.DeckStat, error)
	IncrementDeckTotalCards(ctx context.Context, deckID uuid.UUID) error
	ShiftDeckCardStates(ctx context.Context, arg db.ShiftDeckCardStatesParams) error
	UpdateDeckName(ctx context.Context, deckID uuid.UUID, name string) error
	DeleteDeckStats(ctx context.Context, deckID, userID uuid.UUID) error

	// deck_progress_snapshots
	UpsertDeckProgressSnapshot(ctx context.Context, arg db.UpsertDeckProgressSnapshotParams) error
	ListDeckProgressSnapshots(ctx context.Context, deckID uuid.UUID, from, to time.Time) ([]db.DeckProgressSnapshot, error)
	DeleteDeckProgressSnapshots(ctx context.Context, deckID, userID uuid.UUID) error

	// Reminder cron support
	BumpActivityBucket(ctx context.Context, userID uuid.UUID, hour int16, dayType int16, count int32) error
	ListActivityBuckets(ctx context.Context, userID uuid.UUID) ([]db.UserActivityBucket, error)
	SetOptimalHours(ctx context.Context, userID uuid.UUID, weekday, weekend *int16) error
	ListReminderState(ctx context.Context, onlyActiveStreak bool) ([]ReminderState, error)
}

// ReminderState mirrors the service-level shape so the repository can return
// the union of two queries without leaking sqlc types upward.
type ReminderState struct {
	UserID             uuid.UUID
	CurrentStreak      int32
	LastStudiedDate    *time.Time
	OptimalHourWeekday *int16
	OptimalHourWeekend *int16
	ReminderLocalTime  string
}

type statsRepository struct {
	q *db.Queries
}

func New(database *sql.DB) StatsRepository {
	return &statsRepository{q: db.New(database)}
}

func (r *statsRepository) CreateUserStats(ctx context.Context, userID uuid.UUID, username, avatarURL string) (db.UserStat, error) {
	return r.q.CreateUserStats(ctx, db.CreateUserStatsParams{
		UserID:    userID,
		Username:  sql.NullString{String: username, Valid: username != ""},
		AvatarUrl: sql.NullString{String: avatarURL, Valid: avatarURL != ""},
	})
}

func (r *statsRepository) GetUserStats(ctx context.Context, userID uuid.UUID) (db.UserStat, error) {
	row, err := r.q.GetUserStats(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.UserStat{}, domain.ErrUserStatsNotFound
	}
	return row, err
}

func (r *statsRepository) IncrementReview(ctx context.Context, arg db.IncrementReviewParams) error {
	return r.q.IncrementReview(ctx, arg)
}

func (r *statsRepository) UpdateStreak(ctx context.Context, userID uuid.UUID, streak int32, lastDate time.Time) error {
	return r.q.UpdateStreak(ctx, db.UpdateStreakParams{
		UserID:          userID,
		CurrentStreak:   streak,
		LastStudiedDate: sql.NullTime{Time: lastDate, Valid: true},
	})
}

func (r *statsRepository) IncrementUserCards(ctx context.Context, userID uuid.UUID) error {
	return r.q.IncrementUserCards(ctx, userID)
}

func (r *statsRepository) DecrementUserCards(ctx context.Context, userID uuid.UUID, count int32) error {
	return r.q.DecrementUserCards(ctx, db.DecrementUserCardsParams{
		UserID:     userID,
		TotalCards: count,
	})
}

func (r *statsRepository) UpdateUserProfile(ctx context.Context, userID uuid.UUID, username, avatarURL string) error {
	return r.q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		UserID:    userID,
		Username:  sql.NullString{String: username, Valid: username != ""},
		AvatarUrl: sql.NullString{String: avatarURL, Valid: avatarURL != ""},
	})
}

func (r *statsRepository) UpsertDailyStats(ctx context.Context, arg db.UpsertDailyStatsParams) error {
	return r.q.UpsertDailyStats(ctx, arg)
}

func (r *statsRepository) ListDailyStats(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]db.DailyStat, error) {
	return r.q.ListDailyStats(ctx, db.ListDailyStatsParams{
		UserID:      userID,
		StudyDate:   from,
		StudyDate_2: to,
	})
}

func (r *statsRepository) CreateDeckStats(ctx context.Context, deckID, userID uuid.UUID, deckName string) error {
	return r.q.CreateDeckStats(ctx, db.CreateDeckStatsParams{
		DeckID:   deckID,
		UserID:   userID,
		DeckName: sql.NullString{String: deckName, Valid: deckName != ""},
	})
}

func (r *statsRepository) GetDeckStats(ctx context.Context, deckID uuid.UUID) (db.DeckStat, error) {
	row, err := r.q.GetDeckStats(ctx, deckID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.DeckStat{}, domain.ErrDeckStatsNotFound
	}
	return row, err
}

func (r *statsRepository) ListDeckStatsByUser(ctx context.Context, userID uuid.UUID) ([]db.DeckStat, error) {
	return r.q.ListDeckStatsByUser(ctx, userID)
}

func (r *statsRepository) IncrementDeckTotalCards(ctx context.Context, deckID uuid.UUID) error {
	return r.q.IncrementDeckTotalCards(ctx, deckID)
}

func (r *statsRepository) ShiftDeckCardStates(ctx context.Context, arg db.ShiftDeckCardStatesParams) error {
	return r.q.ShiftDeckCardStates(ctx, arg)
}

func (r *statsRepository) UpdateDeckName(ctx context.Context, deckID uuid.UUID, name string) error {
	return r.q.UpdateDeckName(ctx, db.UpdateDeckNameParams{
		DeckID:   deckID,
		DeckName: sql.NullString{String: name, Valid: name != ""},
	})
}

func (r *statsRepository) DeleteDeckStats(ctx context.Context, deckID, userID uuid.UUID) error {
	return r.q.DeleteDeckStats(ctx, db.DeleteDeckStatsParams{
		DeckID: deckID,
		UserID: userID,
	})
}

func (r *statsRepository) UpsertDeckProgressSnapshot(ctx context.Context, arg db.UpsertDeckProgressSnapshotParams) error {
	return r.q.UpsertDeckProgressSnapshot(ctx, arg)
}

func (r *statsRepository) ListDeckProgressSnapshots(ctx context.Context, deckID uuid.UUID, from, to time.Time) ([]db.DeckProgressSnapshot, error) {
	return r.q.ListDeckProgressSnapshots(ctx, db.ListDeckProgressSnapshotsParams{
		DeckID:         deckID,
		SnapshotDate:   from,
		SnapshotDate_2: to,
	})
}

func (r *statsRepository) DeleteDeckProgressSnapshots(ctx context.Context, deckID, userID uuid.UUID) error {
	return r.q.DeleteDeckProgressSnapshots(ctx, db.DeleteDeckProgressSnapshotsParams{
		DeckID: deckID,
		UserID: userID,
	})
}

// ── Reminder cron ───────────────────────────────────────────────────────────

func (r *statsRepository) BumpActivityBucket(ctx context.Context, userID uuid.UUID, hour int16, dayType int16, count int32) error {
	return r.q.BumpActivityBucket(ctx, db.BumpActivityBucketParams{
		UserID:      userID,
		HourOfDay:   hour,
		DayType:     dayType,
		ReviewCount: count,
	})
}

func (r *statsRepository) ListActivityBuckets(ctx context.Context, userID uuid.UUID) ([]db.UserActivityBucket, error) {
	return r.q.GetActivityBuckets(ctx, userID)
}

func (r *statsRepository) SetOptimalHours(ctx context.Context, userID uuid.UUID, weekday, weekend *int16) error {
	var wd, we sql.NullInt16
	if weekday != nil {
		wd = sql.NullInt16{Int16: *weekday, Valid: true}
	}
	if weekend != nil {
		we = sql.NullInt16{Int16: *weekend, Valid: true}
	}
	return r.q.SetOptimalHours(ctx, db.SetOptimalHoursParams{
		UserID:             userID,
		OptimalHourWeekday: wd,
		OptimalHourWeekend: we,
	})
}

// ListReminderState calls one of the two sqlc-generated queries depending on
// the filter flag and converts to the repository-level struct. The two queries
// return identical column shapes.
func (r *statsRepository) ListReminderState(ctx context.Context, onlyActiveStreak bool) ([]ReminderState, error) {
	if onlyActiveStreak {
		rows, err := r.q.ListUsersWithActiveStreak(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]ReminderState, 0, len(rows))
		for _, row := range rows {
			out = append(out, reminderRowToState(
				row.UserID, row.CurrentStreak, row.LastStudiedDate,
				row.OptimalHourWeekday, row.OptimalHourWeekend, row.ReminderLocalTime,
			))
		}
		return out, nil
	}
	rows, err := r.q.ListUsersForStudyReminder(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ReminderState, 0, len(rows))
	for _, row := range rows {
		out = append(out, reminderRowToState(
			row.UserID, row.CurrentStreak, row.LastStudiedDate,
			row.OptimalHourWeekday, row.OptimalHourWeekend, row.ReminderLocalTime,
		))
	}
	return out, nil
}

func reminderRowToState(
	userID uuid.UUID,
	streak int32,
	lastDate sql.NullTime,
	hourWD, hourWE sql.NullInt16,
	reminderTime string,
) ReminderState {
	st := ReminderState{
		UserID:            userID,
		CurrentStreak:     streak,
		ReminderLocalTime: reminderTime,
	}
	if lastDate.Valid {
		t := lastDate.Time
		st.LastStudiedDate = &t
	}
	if hourWD.Valid {
		h := hourWD.Int16
		st.OptimalHourWeekday = &h
	}
	if hourWE.Valid {
		h := hourWE.Int16
		st.OptimalHourWeekend = &h
	}
	return st
}
