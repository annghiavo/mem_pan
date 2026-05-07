package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/domain"
)

type DeckSettingsRepository interface {
	GetDeckSettings(ctx context.Context, userID, deckID uuid.UUID) (db.DeckStudySetting, error)
	UpsertDeckSettings(ctx context.Context, arg db.UpsertDeckStudySettingsParams) (db.DeckStudySetting, error)
}

type deckSettingsRepository struct {
	q *db.Queries
}

func NewDeckSettingsRepository(d *sql.DB) DeckSettingsRepository {
	return &deckSettingsRepository{q: db.New(d)}
}

func (r *deckSettingsRepository) GetDeckSettings(ctx context.Context, userID, deckID uuid.UUID) (db.DeckStudySetting, error) {
	s, err := r.q.GetDeckStudySettings(ctx, db.GetDeckStudySettingsParams{
		UserID: userID,
		DeckID: deckID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return db.DeckStudySetting{}, domain.ErrSettingsNotFound
	}
	return s, err
}

func (r *deckSettingsRepository) UpsertDeckSettings(ctx context.Context, arg db.UpsertDeckStudySettingsParams) (db.DeckStudySetting, error) {
	return r.q.UpsertDeckStudySettings(ctx, arg)
}
