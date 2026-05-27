package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"mem_pan/services/study-service/internal/db"
)

type RevlogRepository interface {
	InsertRevlog(ctx context.Context, arg db.InsertRevlogParams) (db.Revlog, error)
	// ListUsersWithMinReviews returns users who have at least minReviews logs —
	// the candidates for daily FSRS weight optimization.
	ListUsersWithMinReviews(ctx context.Context, minReviews int64) ([]db.ListUsersWithMinReviewsRow, error)
	// ListReviewLogsForOptimize returns one user's review logs (oldest first) as
	// training samples for the optimizer.
	ListReviewLogsForOptimize(ctx context.Context, userID uuid.UUID) ([]db.ListReviewLogsForOptimizeRow, error)
}

type revlogRepository struct {
	q *db.Queries
}

func NewRevlogRepository(database *sql.DB) RevlogRepository {
	return &revlogRepository{q: db.New(database)}
}

func (r *revlogRepository) InsertRevlog(ctx context.Context, arg db.InsertRevlogParams) (db.Revlog, error) {
	return r.q.InsertRevlog(ctx, arg)
}

func (r *revlogRepository) ListUsersWithMinReviews(ctx context.Context, minReviews int64) ([]db.ListUsersWithMinReviewsRow, error) {
	return r.q.ListUsersWithMinReviews(ctx, minReviews)
}

func (r *revlogRepository) ListReviewLogsForOptimize(ctx context.Context, userID uuid.UUID) ([]db.ListReviewLogsForOptimizeRow, error) {
	return r.q.ListReviewLogsForOptimize(ctx, userID)
}
