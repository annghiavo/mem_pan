package subscriber

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/stats-service/internal/db"
	"mem_pan/services/stats-service/internal/domain"
	"mem_pan/services/stats-service/internal/events"
	"mem_pan/services/stats-service/internal/repository"
)

const masteredStabilityThreshold = 21.0

type Handler struct {
	repo repository.StatsRepository
}

func NewHandler(repo repository.StatsRepository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Dispatch(ctx context.Context, eventType string, data []byte) error {
	switch eventType {
	case events.TypeUserRegistered:
		return h.handleUserRegistered(ctx, data)
	case events.TypeDeckCreated:
		return h.handleDeckCreated(ctx, data)
	case events.TypeDeckUpdated:
		return h.handleDeckUpdated(ctx, data)
	case events.TypeDeckDeleted:
		return h.handleDeckDeleted(ctx, data)
	case events.TypeCardCreated:
		return h.handleCardCreated(ctx, data)
	case events.TypeCardReviewed:
		return h.handleCardReviewed(ctx, data)
	default:
		log.Printf("[stats] unknown event type %q — skipping", eventType)
		return nil
	}
}

func (h *Handler) handleUserRegistered(ctx context.Context, data []byte) error {
	var e events.UserRegistered
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}

	userID, err := uuid.Parse(e.UserID)
	if err != nil {
		return err
	}

	_, err = h.repo.CreateUserStats(ctx, userID, e.Username, e.AvatarURL)
	if err != nil {
		log.Printf("[stats] CreateUserStats %s: %v", userID, err)
	}
	return err
}

func (h *Handler) handleDeckCreated(ctx context.Context, data []byte) error {
	var e events.DeckCreated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}

	deckID, err := uuid.Parse(e.DeckID)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(e.UserID)
	if err != nil {
		return err
	}

	return h.repo.CreateDeckStats(ctx, deckID, userID, e.DeckName)
}

func (h *Handler) handleDeckUpdated(ctx context.Context, data []byte) error {
	var e events.DeckUpdated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}

	deckID, err := uuid.Parse(e.DeckID)
	if err != nil {
		return err
	}

	return h.repo.UpdateDeckName(ctx, deckID, e.DeckName)
}

// handleDeckDeleted removes the deck-scoped aggregates when a deck is deleted.
// It deletes deck_stats and deck_progress_snapshots for the deck, and subtracts
// the deck's card count from the user's lifetime total_cards. The user's
// learning achievements (streak, total_reviews, study time, daily heatmap,
// activity buckets) live in separate rows and are deliberately left untouched.
//
// Ordering matters: read the card count from deck_stats *before* deleting that
// row, otherwise the decrement amount is lost. All operations are idempotent —
// a redelivered event finds deck_stats already gone (GetDeckStats returns
// ErrDeckStatsNotFound) and skips the decrement, and the DELETEs are no-ops.
func (h *Handler) handleDeckDeleted(ctx context.Context, data []byte) error {
	var e events.DeckDeleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}

	deckID, err := uuid.Parse(e.DeckID)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(e.UserID)
	if err != nil {
		return err
	}

	// Read the deck's card count before deleting deck_stats so we can adjust the
	// user's lifetime collection size. If the deck_stats row is already gone
	// (duplicate event, or the deck was created before stats-service existed),
	// skip the decrement rather than failing.
	if ds, err := h.repo.GetDeckStats(ctx, deckID); err == nil {
		if ds.TotalCards > 0 {
			if err := h.repo.DecrementUserCards(ctx, userID, ds.TotalCards); err != nil {
				return err
			}
		}
	} else if !errors.Is(err, domain.ErrDeckStatsNotFound) {
		return err
	}

	if err := h.repo.DeleteDeckProgressSnapshots(ctx, deckID, userID); err != nil {
		return err
	}
	return h.repo.DeleteDeckStats(ctx, deckID, userID)
}

func (h *Handler) handleCardCreated(ctx context.Context, data []byte) error {
	var e events.CardCreated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}

	deckID, err := uuid.Parse(e.DeckID)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(e.UserID)
	if err != nil {
		return err
	}

	if err := h.repo.IncrementDeckTotalCards(ctx, deckID); err != nil {
		return err
	}
	return h.repo.IncrementUserCards(ctx, userID)
}

func (h *Handler) handleCardReviewed(ctx context.Context, data []byte) error {
	var e events.CardReviewed
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}

	userID, err := uuid.Parse(e.UserID)
	if err != nil {
		return err
	}
	deckID, err := uuid.Parse(e.DeckID)
	if err != nil {
		return err
	}

	// Ensure user_stats row exists for legacy users (registered before stats-service
	// was deployed, or where user.registered event was lost). Without this, the
	// UPDATEs below silently do nothing and streak/counters stay at 0.
	if _, err := h.repo.CreateUserStats(ctx, userID, "", ""); err != nil {
		log.Printf("[stats] ensure user_stats %s: %v", userID, err)
	}

	isCorrect := e.Rating >= 3

	// 1. Increment overall review counters
	var correct, incorrect int32
	if isCorrect {
		correct = 1
	} else {
		incorrect = 1
	}
	if err := h.repo.IncrementReview(ctx, db.IncrementReviewParams{
		UserID:           userID,
		TotalReviews:     1,
		TotalStudyTimeMs: e.DurationMs,
		TotalCorrect:     correct,
		TotalIncorrect:   incorrect,
	}); err != nil {
		return err
	}

	// 2. Update streak based on review time, in the user's local timezone.
	// Day boundary follows the user's local midnight, not UTC.
	loc := loadTZ(e.Timezone)
	if err := h.updateStreak(ctx, userID, e.ReviewTime, loc); err != nil {
		log.Printf("[stats] updateStreak %s: %v", userID, err)
	}

	// 2a. Bump the activity histogram for optimal-hour prediction.
	localReview := e.ReviewTime.In(loc)
	dayType := int16(0)
	if wd := localReview.Weekday(); wd == time.Saturday || wd == time.Sunday {
		dayType = 1
	}
	if err := h.repo.BumpActivityBucket(ctx, userID, int16(localReview.Hour()), dayType, 1); err != nil {
		log.Printf("[stats] bumpActivityBucket %s: %v", userID, err)
	}

	// 3. Upsert daily stats
	studyDate := e.ReviewTime.UTC().Truncate(24 * time.Hour)
	var newCardsCount int32
	if e.IsNewCard {
		newCardsCount = 1
	}
	if err := h.repo.UpsertDailyStats(ctx, db.UpsertDailyStatsParams{
		UserID:        userID,
		StudyDate:     studyDate,
		ReviewsCount:  1,
		NewCardsCount: newCardsCount,
		StudyTimeMs:   e.DurationMs,
		CorrectCount:  correct,
	}); err != nil {
		return err
	}

	// 4. Shift card state counts in deck_stats
	newDelta, learningDelta, reviewDelta, masteredDelta := computeStateDelta(
		e.StateBefore, e.StateAfter, e.StabilityAfter,
	)
	if newDelta != 0 || learningDelta != 0 || reviewDelta != 0 || masteredDelta != 0 {
		if err := h.repo.ShiftDeckCardStates(ctx, db.ShiftDeckCardStatesParams{
			DeckID:        deckID,
			NewCards:      newDelta,
			LearningCards: learningDelta,
			ReviewCards:   reviewDelta,
			MasteredCards: masteredDelta,
		}); err != nil {
			log.Printf("[stats] ShiftDeckCardStates %s: %v", deckID, err)
		}
	}

	// 5. Update today's deck progress snapshot from latest deck_stats
	if err := h.snapshotDeckProgress(ctx, deckID, userID, studyDate); err != nil {
		log.Printf("[stats] snapshotDeckProgress %s: %v", deckID, err)
	}

	return nil
}

func (h *Handler) updateStreak(ctx context.Context, userID uuid.UUID, reviewTime time.Time, loc *time.Location) error {
	row, err := h.repo.GetUserStats(ctx, userID)
	if err != nil {
		return err
	}

	// "today" represents the user's local calendar date, but constructed as UTC
	// midnight on that date. The pgx driver (QueryExecModeExec) sends time.Time
	// as TIMESTAMPTZ; Postgres then casts to DATE in the session tz (UTC), which
	// would shift the date backwards by the tz offset if we used a non-UTC zone
	// here. Building UTC midnight from the local Y/M/D keeps storage and reads
	// symmetric (Postgres DATE always decodes as UTC midnight in pgx too).
	localReview := reviewTime.In(loc)
	y, m, d := localReview.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	newStreak := computeStreak(row.LastStudiedDate, today, row.CurrentStreak)

	return h.repo.UpdateStreak(ctx, userID, newStreak, today)
}

func loadTZ(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func computeStreak(last sql.NullTime, today time.Time, current int32) int32 {
	if !last.Valid {
		return 1
	}
	// last comes from a Postgres DATE (decoded as UTC midnight), today is local
	// midnight in the user's tz. Compare by Y/M/D, not instant — otherwise a
	// non-UTC user's streak resets to 1 on every review.
	ly, lm, ld := last.Time.UTC().Date()
	ty, tm, td := today.Date()
	if ly == ty && lm == tm && ld == td {
		return current
	}
	yy, ym, yd := today.AddDate(0, 0, -1).Date()
	if ly == yy && lm == ym && ld == yd {
		return current + 1
	}
	return 1
}

// computeStateDelta returns (newDelta, learningDelta, reviewDelta, masteredDelta)
// representing how each bucket in deck_stats should change.
func computeStateDelta(before, after string, stabilityAfter float64) (newD, learnD, reviewD, masteredD int32) {
	// Decrement the "before" bucket
	switch before {
	case "new":
		newD = -1
	case "learning", "relearning":
		learnD = -1
	case "review":
		if stabilityAfter < masteredStabilityThreshold {
			reviewD = -1
		} else {
			masteredD = -1
		}
	}

	// Increment the "after" bucket
	switch after {
	case "new":
		newD++
	case "learning", "relearning":
		learnD++
	case "review":
		if stabilityAfter >= masteredStabilityThreshold {
			masteredD++
		} else {
			reviewD++
		}
	}

	return
}

func (h *Handler) snapshotDeckProgress(ctx context.Context, deckID, userID uuid.UUID, date time.Time) error {
	ds, err := h.repo.GetDeckStats(ctx, deckID)
	if err != nil {
		return err
	}
	return h.repo.UpsertDeckProgressSnapshot(ctx, db.UpsertDeckProgressSnapshotParams{
		DeckID:        deckID,
		UserID:        userID,
		SnapshotDate:  date,
		NewCount:      ds.NewCards,
		LearningCount: ds.LearningCards,
		ReviewCount:   ds.ReviewCards,
		MasteredCount: ds.MasteredCards,
	})
}
