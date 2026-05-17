package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
	AdminUpdateDeckStatus(ctx context.Context, arg db.AdminUpdateDeckStatusParams) (db.Deck, error)
	AdminListDecks(ctx context.Context, arg db.AdminListDecksParams) ([]db.Deck, error)
	AdminCountDecks(ctx context.Context, statusFilter sql.NullString) (int64, error)
	IncrementCardCount(ctx context.Context, deckID uuid.UUID) error
	DecrementCardCount(ctx context.Context, deckID uuid.UUID) error
	CloneDeck(ctx context.Context, src db.Deck, newOwnerID uuid.UUID, newName string) (db.Deck, []db.ListCardsByDeckRow, error)
}

type deckRepository struct {
	db *sql.DB
	q  *db.Queries
}

func NewDeckRepository(database *sql.DB) DeckRepository {
	return &deckRepository{db: database, q: db.New(database)}
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

func (r *deckRepository) AdminUpdateDeckStatus(ctx context.Context, arg db.AdminUpdateDeckStatusParams) (db.Deck, error) {
	d, err := r.q.AdminUpdateDeckStatus(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	return d, err
}

func (r *deckRepository) AdminListDecks(ctx context.Context, arg db.AdminListDecksParams) ([]db.Deck, error) {
	return r.q.AdminListDecks(ctx, arg)
}

func (r *deckRepository) AdminCountDecks(ctx context.Context, statusFilter sql.NullString) (int64, error) {
	return r.q.AdminCountDecks(ctx, statusFilter)
}

func (r *deckRepository) IncrementCardCount(ctx context.Context, deckID uuid.UUID) error {
	return r.q.IncrementCardCount(ctx, deckID)
}

func (r *deckRepository) DecrementCardCount(ctx context.Context, deckID uuid.UUID) error {
	return r.q.DecrementCardCount(ctx, deckID)
}

// CloneDeck duplicates a source deck under a new owner: creates a new deck row
// (is_public=FALSE, cloned_from=src.DeckID), copies every note+card with the
// same position, and sets card_count. Runs inside a single transaction so a
// partial clone is impossible.
//
// Returns the new deck and the freshly cloned cards (with note content) so the
// caller can emit deck.created + card.created events for search indexing.
func (r *deckRepository) CloneDeck(ctx context.Context, src db.Deck, newOwnerID uuid.UUID, newName string) (db.Deck, []db.ListCardsByDeckRow, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return db.Deck{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := r.q.WithTx(tx)

	newDeck, err := q.CloneDeck(ctx, db.CloneDeckParams{
		UserID:      newOwnerID,
		Name:        newName,
		Description: src.Description,
		ClonedFrom:  uuid.NullUUID{UUID: src.DeckID, Valid: true},
	})
	if err != nil {
		return db.Deck{}, nil, fmt.Errorf("insert cloned deck: %w", err)
	}

	sourceCards, err := q.ListCardsByDeck(ctx, src.DeckID)
	if err != nil {
		return db.Deck{}, nil, fmt.Errorf("list source cards: %w", err)
	}

	clonedCards := make([]db.ListCardsByDeckRow, 0, len(sourceCards))
	for _, c := range sourceCards {
		newNote, err := q.CreateNote(ctx, db.CreateNoteParams{
			UserID:       newOwnerID,
			ContentFront: c.ContentFront,
			ContentBack:  c.ContentBack,
			ImageUrl:     c.ImageUrl,
			LangFront:    c.LangFront,
			LangBack:     c.LangBack,
		})
		if err != nil {
			return db.Deck{}, nil, fmt.Errorf("clone note: %w", err)
		}
		newCard, err := q.CreateCard(ctx, db.CreateCardParams{
			UserID:   newOwnerID,
			DeckID:   newDeck.DeckID,
			NoteID:   newNote.NoteID,
			Position: c.Position,
		})
		if err != nil {
			return db.Deck{}, nil, fmt.Errorf("clone card: %w", err)
		}
		clonedCards = append(clonedCards, db.ListCardsByDeckRow{
			CardID:       newCard.CardID,
			UserID:       newCard.UserID,
			DeckID:       newCard.DeckID,
			NoteID:       newCard.NoteID,
			Position:     newCard.Position,
			CreatedAt:    newCard.CreatedAt,
			ContentFront: newNote.ContentFront,
			ContentBack:  newNote.ContentBack,
			ImageUrl:     newNote.ImageUrl,
			LangFront:    newNote.LangFront,
			LangBack:     newNote.LangBack,
		})
	}

	if cnt := int32(len(clonedCards)); cnt > 0 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE decks SET card_count = $1, updated_at = now() WHERE deck_id = $2`,
			cnt, newDeck.DeckID,
		); err != nil {
			return db.Deck{}, nil, fmt.Errorf("update card_count: %w", err)
		}
		newDeck.CardCount = cnt
	}

	if err := tx.Commit(); err != nil {
		return db.Deck{}, nil, fmt.Errorf("commit clone: %w", err)
	}
	return newDeck, clonedCards, nil
}

// DefaultSettings returns the default deck settings as JSON.
func DefaultSettings() json.RawMessage {
	return json.RawMessage(`{"quiz_type":"multiple_choice","answer_side":"back","strict_typing":false,"partial_correct":true,"new_cards_per_day":20,"reviews_per_day":200}`)
}
