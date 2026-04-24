package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/study-service/internal/db"
)

type FsrsWeightsRepository interface {
	GetActiveWeights(ctx context.Context, userID uuid.UUID) (db.UserFsrsWeight, error)
	DeactivateWeights(ctx context.Context, userID uuid.UUID) error
	GetNextWeightVersion(ctx context.Context, userID uuid.UUID) (int32, error)
	InsertWeights(ctx context.Context, arg db.InsertWeightsParams) (db.UserFsrsWeight, error)
}

type fsrsWeightsRepository struct {
	q *db.Queries
}

func NewFsrsWeightsRepository(database *sql.DB) FsrsWeightsRepository {
	return &fsrsWeightsRepository{q: db.New(database)}
}

func (r *fsrsWeightsRepository) GetActiveWeights(ctx context.Context, userID uuid.UUID) (db.UserFsrsWeight, error) {
	w, err := r.q.GetActiveWeights(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.UserFsrsWeight{}, nil
	}
	return w, err
}

func (r *fsrsWeightsRepository) DeactivateWeights(ctx context.Context, userID uuid.UUID) error {
	return r.q.DeactivateWeights(ctx, userID)
}

func (r *fsrsWeightsRepository) GetNextWeightVersion(ctx context.Context, userID uuid.UUID) (int32, error) {
	return r.q.GetNextWeightVersion(ctx, userID)
}

func (r *fsrsWeightsRepository) InsertWeights(ctx context.Context, arg db.InsertWeightsParams) (db.UserFsrsWeight, error) {
	return r.q.InsertWeights(ctx, arg)
}
