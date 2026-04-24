package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
)

type DeckRepository interface {
	CreateDeck(ctx context.Context, arg db.CreateDeckParams) (db.Deck, error)
	GetDeckByID(ctx context.Context, id uuid.UUID) (db.Deck, error)
	ListDecksByUser(ctx context.Context, arg db.ListDecksByUserParams) ([]db.Deck, error)
	CountDecksByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListPublicDecks(ctx context.Context, arg db.ListPublicDecksParams) ([]db.Deck, error)
	CountPublicDecks(ctx context.Context) (int64, error)
	UpdateDeck(ctx context.Context, arg db.UpdateDeckParams) (db.Deck, error)
	UpdateDeckSettings(ctx context.Context, arg db.UpdateDeckSettingsParams) (db.Deck, error)
	UpdateDeckVisibility(ctx context.Context, arg db.UpdateDeckVisibilityParams) (db.Deck, error)
	SoftDeleteDeck(ctx context.Context, arg db.SoftDeleteDeckParams) error
	IncrementCardCount(ctx context.Context, deckID uuid.UUID) error
	DecrementCardCount(ctx context.Context, deckID uuid.UUID) error
	CloneDeck(ctx context.Context, arg db.CloneDeckParams) (db.Deck, error)
}

type deckRepository struct {
	q *db.Queries
}

func NewDeckRepository(database *sql.DB) DeckRepository {
	return &deckRepository{q: db.New(database)}
}

func (r *deckRepository) CreateDeck(ctx context.Context, arg db.CreateDeckParams) (db.Deck, error) {
	return r.q.CreateDeck(ctx, arg)
}

func (r *deckRepository) GetDeckByID(ctx context.Context, id uuid.UUID) (db.Deck, error) {
	d, err := r.q.GetDeckByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	return d, err
}

func (r *deckRepository) ListDecksByUser(ctx context.Context, arg db.ListDecksByUserParams) ([]db.Deck, error) {
	return r.q.ListDecksByUser(ctx, arg)
}

func (r *deckRepository) CountDecksByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.CountDecksByUser(ctx, userID)
}

func (r *deckRepository) ListPublicDecks(ctx context.Context, arg db.ListPublicDecksParams) ([]db.Deck, error) {
	return r.q.ListPublicDecks(ctx, arg)
}

func (r *deckRepository) CountPublicDecks(ctx context.Context) (int64, error) {
	return r.q.CountPublicDecks(ctx)
}

func (r *deckRepository) UpdateDeck(ctx context.Context, arg db.UpdateDeckParams) (db.Deck, error) {
	d, err := r.q.UpdateDeck(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	return d, err
}

func (r *deckRepository) UpdateDeckSettings(ctx context.Context, arg db.UpdateDeckSettingsParams) (db.Deck, error) {
	d, err := r.q.UpdateDeckSettings(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	return d, err
}

func (r *deckRepository) UpdateDeckVisibility(ctx context.Context, arg db.UpdateDeckVisibilityParams) (db.Deck, error) {
	d, err := r.q.UpdateDeckVisibility(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	return d, err
}

func (r *deckRepository) SoftDeleteDeck(ctx context.Context, arg db.SoftDeleteDeckParams) error {
	return r.q.SoftDeleteDeck(ctx, arg)
}

func (r *deckRepository) IncrementCardCount(ctx context.Context, deckID uuid.UUID) error {
	return r.q.IncrementCardCount(ctx, deckID)
}

func (r *deckRepository) DecrementCardCount(ctx context.Context, deckID uuid.UUID) error {
	return r.q.DecrementCardCount(ctx, deckID)
}

func (r *deckRepository) CloneDeck(ctx context.Context, arg db.CloneDeckParams) (db.Deck, error) {
	return r.q.CloneDeck(ctx, arg)
}

// DefaultSettings returns the default deck settings as JSON.
func DefaultSettings() json.RawMessage {
	return json.RawMessage(`{"quiz_type":"multiple_choice","answer_side":"back","strict_typing":false,"partial_correct":true,"new_cards_per_day":20,"reviews_per_day":200}`)
}
