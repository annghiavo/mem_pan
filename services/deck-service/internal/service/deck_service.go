package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/repository"
)

type DeckSettings struct {
	QuizType        string `json:"quiz_type"`
	AnswerSide      string `json:"answer_side"`
	StrictTyping    bool   `json:"strict_typing"`
	PartialCorrect  bool   `json:"partial_correct"`
	NewCardsPerDay  int32  `json:"new_cards_per_day"`
	ReviewsPerDay   int32  `json:"reviews_per_day"`
}

type CreateDeckParams struct {
	UserID      uuid.UUID
	Name        string
	Description *string
	IsPublic    bool
}

type UpdateDeckParams struct {
	DeckID      uuid.UUID
	UserID      uuid.UUID
	Name        *string
	Description *string
}

type ListDecksParams struct {
	UserID uuid.UUID
	Limit  int32
	Offset int32
}

type ListPublicDecksParams struct {
	Limit  int32
	Offset int32
}

type DecksPage struct {
	Decks []db.Deck
	Total int64
}

type DeckStats struct {
	DeckID     uuid.UUID
	TotalCards int64
}

type DeckService interface {
	CreateDeck(ctx context.Context, p CreateDeckParams) (db.Deck, error)
	GetDeck(ctx context.Context, deckID, userID uuid.UUID, publicOK bool) (db.Deck, error)
	ListDecks(ctx context.Context, p ListDecksParams) (DecksPage, error)
	ListPublicDecks(ctx context.Context, p ListPublicDecksParams) (DecksPage, error)
	UpdateDeck(ctx context.Context, p UpdateDeckParams) (db.Deck, error)
	DeleteDeck(ctx context.Context, deckID, userID uuid.UUID) error
	UpdateSettings(ctx context.Context, deckID, userID uuid.UUID, settings DeckSettings) (db.Deck, error)
	UpdateVisibility(ctx context.Context, deckID, userID uuid.UUID, isPublic bool) (db.Deck, error)
	CloneDeck(ctx context.Context, sourceDeckID, newOwnerID uuid.UUID) (db.Deck, error)
	GetStats(ctx context.Context, deckID, userID uuid.UUID) (DeckStats, error)
}

type deckService struct {
	deckRepo repository.DeckRepository
	cardRepo repository.CardRepository
}

func NewDeckService(deckRepo repository.DeckRepository, cardRepo repository.CardRepository) DeckService {
	return &deckService{deckRepo: deckRepo, cardRepo: cardRepo}
}

func (s *deckService) CreateDeck(ctx context.Context, p CreateDeckParams) (db.Deck, error) {
	return s.deckRepo.CreateDeck(ctx, db.CreateDeckParams{
		UserID:      p.UserID,
		Name:        p.Name,
		Description: nullStr(p.Description),
		IsPublic:    p.IsPublic,
	})
}

func (s *deckService) GetDeck(ctx context.Context, deckID, userID uuid.UUID, publicOK bool) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.Status == string(db.ContentStatusDeleted) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	if deck.UserID != userID {
		if !publicOK || !deck.IsPublic {
			return db.Deck{}, domain.ErrForbidden
		}
	}
	return deck, nil
}

func (s *deckService) ListDecks(ctx context.Context, p ListDecksParams) (DecksPage, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	decks, err := s.deckRepo.ListDecksByUser(ctx, db.ListDecksByUserParams{
		UserID: p.UserID,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return DecksPage{}, err
	}
	total, err := s.deckRepo.CountDecksByUser(ctx, p.UserID)
	if err != nil {
		return DecksPage{}, err
	}
	return DecksPage{Decks: decks, Total: total}, nil
}

func (s *deckService) ListPublicDecks(ctx context.Context, p ListPublicDecksParams) (DecksPage, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	decks, err := s.deckRepo.ListPublicDecks(ctx, db.ListPublicDecksParams{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return DecksPage{}, err
	}
	total, err := s.deckRepo.CountPublicDecks(ctx)
	if err != nil {
		return DecksPage{}, err
	}
	return DecksPage{Decks: decks, Total: total}, nil
}

func (s *deckService) UpdateDeck(ctx context.Context, p UpdateDeckParams) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, p.DeckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.UserID != p.UserID {
		return db.Deck{}, domain.ErrForbidden
	}
	return s.deckRepo.UpdateDeck(ctx, db.UpdateDeckParams{
		DeckID:      p.DeckID,
		UserID:      p.UserID,
		Name:        nullStr(p.Name),
		Description: nullStr(p.Description),
	})
}

func (s *deckService) DeleteDeck(ctx context.Context, deckID, userID uuid.UUID) error {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return err
	}
	if deck.UserID != userID {
		return domain.ErrForbidden
	}
	return s.deckRepo.SoftDeleteDeck(ctx, db.SoftDeleteDeckParams{
		DeckID: deckID,
		UserID: userID,
	})
}

func (s *deckService) UpdateSettings(ctx context.Context, deckID, userID uuid.UUID, settings DeckSettings) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.UserID != userID {
		return db.Deck{}, domain.ErrForbidden
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return db.Deck{}, fmt.Errorf("marshal settings: %w", err)
	}
	return s.deckRepo.UpdateDeckSettings(ctx, db.UpdateDeckSettingsParams{
		DeckID:   deckID,
		Settings: raw,
	})
}

func (s *deckService) UpdateVisibility(ctx context.Context, deckID, userID uuid.UUID, isPublic bool) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.UserID != userID {
		return db.Deck{}, domain.ErrForbidden
	}
	return s.deckRepo.UpdateDeckVisibility(ctx, db.UpdateDeckVisibilityParams{
		DeckID:   deckID,
		UserID:   userID,
		IsPublic: isPublic,
	})
}

func (s *deckService) CloneDeck(ctx context.Context, sourceDeckID, newOwnerID uuid.UUID) (db.Deck, error) {
	src, err := s.deckRepo.GetDeckByID(ctx, sourceDeckID)
	if err != nil {
		return db.Deck{}, err
	}
	if src.Status == string(db.ContentStatusDeleted) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	if !src.IsPublic && src.UserID != newOwnerID {
		return db.Deck{}, domain.ErrForbidden
	}
	clonedName := "Copy of " + src.Name
	return s.deckRepo.CloneDeck(ctx, db.CloneDeckParams{
		UserID:      newOwnerID,
		Name:        clonedName,
		Description: src.Description,
		ClonedFrom:  uuid.NullUUID{UUID: sourceDeckID, Valid: true},
	})
}

func (s *deckService) GetStats(ctx context.Context, deckID, userID uuid.UUID) (DeckStats, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return DeckStats{}, err
	}
	if deck.UserID != userID && !deck.IsPublic {
		return DeckStats{}, domain.ErrForbidden
	}
	total, err := s.cardRepo.CountCardsByDeck(ctx, deckID)
	if err != nil {
		return DeckStats{}, err
	}
	return DeckStats{DeckID: deckID, TotalCards: total}, nil
}

