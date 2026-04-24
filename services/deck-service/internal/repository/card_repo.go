package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
)

type CardRepository interface {
	CreateCard(ctx context.Context, arg db.CreateCardParams) (db.Card, error)
	GetCardByID(ctx context.Context, id uuid.UUID) (db.GetCardByIDRow, error)
	ListCardsByDeck(ctx context.Context, deckID uuid.UUID) ([]db.ListCardsByDeckRow, error)
	DeleteCard(ctx context.Context, arg db.DeleteCardParams) error
	CountCardsByDeck(ctx context.Context, deckID uuid.UUID) (int64, error)
}

type cardRepository struct {
	q *db.Queries
}

func NewCardRepository(database *sql.DB) CardRepository {
	return &cardRepository{q: db.New(database)}
}

func (r *cardRepository) CreateCard(ctx context.Context, arg db.CreateCardParams) (db.Card, error) {
	return r.q.CreateCard(ctx, arg)
}

func (r *cardRepository) GetCardByID(ctx context.Context, id uuid.UUID) (db.GetCardByIDRow, error) {
	c, err := r.q.GetCardByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.GetCardByIDRow{}, domain.ErrCardNotFound
	}
	return c, err
}

func (r *cardRepository) ListCardsByDeck(ctx context.Context, deckID uuid.UUID) ([]db.ListCardsByDeckRow, error) {
	return r.q.ListCardsByDeck(ctx, deckID)
}

func (r *cardRepository) DeleteCard(ctx context.Context, arg db.DeleteCardParams) error {
	return r.q.DeleteCard(ctx, arg)
}

func (r *cardRepository) CountCardsByDeck(ctx context.Context, deckID uuid.UUID) (int64, error) {
	return r.q.CountCardsByDeck(ctx, deckID)
}
