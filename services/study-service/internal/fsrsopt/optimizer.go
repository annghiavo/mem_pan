// Package fsrsopt runs the daily FSRS weight-optimization batch: for every user
// with enough review history, it ships their review logs to moderation-fsrs-service,
// gets back re-tuned weights, and persists them as the new active version.
//
// It is triggered externally (Cloud Scheduler -> HTTP) rather than by an in-process
// ticker, because study-service runs on Cloud Run and scales to zero — a background
// goroutine would not fire while no instance is up.
package fsrsopt

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/moderationclient"
	"mem_pan/services/study-service/internal/repository"
)

// ExpectedWeightCount must match go-fsrs/v4 `Weights [21]float64`. fsrs-optimizer
// 5.5.0 emits FSRS-6 (21 params), so the lengths line up; we still guard, because
// a future optimizer downgrade to 19 (FSRS-5) would silently be ignored by
// fsrs.ParamsFromWeights and leave the user on defaults.
const ExpectedWeightCount = 21

// Optimizer orchestrates one optimization pass.
type Optimizer struct {
	revlogs    repository.RevlogRepository
	weights    repository.FsrsWeightsRepository
	fsrs       moderationclient.Client
	minReviews int64
	maxUsers   int           // 0 = no cap
	perUser    time.Duration // timeout for a single OptimizeWeights call
}

func New(
	revlogs repository.RevlogRepository,
	weights repository.FsrsWeightsRepository,
	fsrs moderationclient.Client,
	minReviews int64,
	maxUsers int,
) *Optimizer {
	return &Optimizer{
		revlogs:    revlogs,
		weights:    weights,
		fsrs:       fsrs,
		minReviews: minReviews,
		maxUsers:   maxUsers,
		perUser:    90 * time.Second,
	}
}

// Summary is the outcome of one RunOnce, returned to the cron caller as JSON.
type Summary struct {
	Eligible   int    `json:"eligible"`    // users with >= minReviews
	Optimized  int    `json:"optimized"`   // weights successfully persisted
	Skipped    int    `json:"skipped"`     // unexpected weight length, etc.
	Failed     int    `json:"failed"`      // optimizer RPC / DB error
	MinReviews int64  `json:"min_reviews"` // threshold used this run
	DurationMs int64  `json:"duration_ms"`
	Note       string `json:"note,omitempty"`
}

// RunOnce processes all eligible users (capped by maxUsers) sequentially.
// Per-user failures are logged and counted, never aborting the whole batch —
// the optimizer is CPU-heavy and one user's bad data shouldn't block the rest.
func (o *Optimizer) RunOnce(ctx context.Context) (Summary, error) {
	start := time.Now()
	s := Summary{MinReviews: o.minReviews}

	users, err := o.revlogs.ListUsersWithMinReviews(ctx, o.minReviews)
	if err != nil {
		return s, err
	}
	s.Eligible = len(users)
	log.Printf("[fsrs-opt] start: %d user(s) with >= %d reviews", s.Eligible, o.minReviews)

	for i, u := range users {
		if o.maxUsers > 0 && i >= o.maxUsers {
			s.Note = "hit max-users cap; remaining users deferred to next run"
			log.Printf("[fsrs-opt] reached max-users cap %d; deferring %d user(s)", o.maxUsers, len(users)-i)
			break
		}
		if err := ctx.Err(); err != nil {
			s.Note = "context cancelled mid-batch"
			break
		}
		switch o.optimizeUser(ctx, u.UserID, u.ReviewCount) {
		case resultOptimized:
			s.Optimized++
		case resultSkipped:
			s.Skipped++
		case resultFailed:
			s.Failed++
		}
	}

	s.DurationMs = time.Since(start).Milliseconds()
	log.Printf("[fsrs-opt] done in %dms: optimized=%d skipped=%d failed=%d (eligible=%d)",
		s.DurationMs, s.Optimized, s.Skipped, s.Failed, s.Eligible)
	return s, nil
}

type userResult int

const (
	resultOptimized userResult = iota
	resultSkipped
	resultFailed
)

func (o *Optimizer) optimizeUser(ctx context.Context, userID uuid.UUID, reviewCount int64) userResult {
	rows, err := o.revlogs.ListReviewLogsForOptimize(ctx, userID)
	if err != nil {
		log.Printf("[fsrs-opt] user=%s: load review logs: %v", userID, err)
		return resultFailed
	}

	logs := make([]moderationclient.ReviewLog, 0, len(rows))
	for _, r := range rows {
		logs = append(logs, moderationclient.ReviewLog{
			CardID:      r.CardID.String(),
			ReviewDate:  r.ReviewTime.Unix(),
			Rating:      int32(r.Rating),
			ElapsedDays: r.ElapsedDays,
		})
	}

	callCtx, cancel := context.WithTimeout(ctx, o.perUser)
	defer cancel()
	res, err := o.fsrs.OptimizeWeights(callCtx, userID.String(), logs)
	if err != nil {
		log.Printf("[fsrs-opt] user=%s: optimize rpc (%d logs): %v", userID, len(logs), err)
		return resultFailed
	}

	if len(res.Weights) != ExpectedWeightCount {
		log.Printf("[fsrs-opt] user=%s: optimizer returned %d weights (want %d, version=%s) — skipping persist",
			userID, len(res.Weights), ExpectedWeightCount, res.FsrsVersion)
		return resultSkipped
	}

	nextVer, err := o.weights.GetNextWeightVersion(ctx, userID)
	if err != nil {
		log.Printf("[fsrs-opt] user=%s: next version: %v", userID, err)
		return resultFailed
	}
	if err := o.weights.DeactivateWeights(ctx, userID); err != nil {
		log.Printf("[fsrs-opt] user=%s: deactivate old weights: %v", userID, err)
		return resultFailed
	}
	_, err = o.weights.InsertWeights(ctx, db.InsertWeightsParams{
		UserID:           userID,
		Version:          nextVer,
		Weights:          pq.Float64Array(res.Weights),
		IsActive:         true,
		TrainedOnReviews: sql.NullInt32{Int32: res.NumReviews, Valid: true},
		TrainingLoss:     sql.NullFloat64{Float64: res.Loss, Valid: true},
	})
	if err != nil {
		log.Printf("[fsrs-opt] user=%s: insert weights v%d: %v", userID, nextVer, err)
		return resultFailed
	}

	log.Printf("[fsrs-opt] user=%s: v%d active (reviews=%d/%d loss=%.4f version=%s)",
		userID, nextVer, res.NumReviews, reviewCount, res.Loss, res.FsrsVersion)
	return resultOptimized
}
