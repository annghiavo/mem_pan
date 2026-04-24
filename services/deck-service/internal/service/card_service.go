package service

import (
	"context"
	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/repository"
)

type CreateCardParams struct {
	UserID       uuid.UUID
	DeckID       uuid.UUID
	ContentFront string
	ContentBack  string
	ImageURL     *string
	Position     int32
}

type UpdateCardParams struct {
	CardID       uuid.UUID
	UserID       uuid.UUID
	ContentFront *string
	ContentBack  *string
	ImageURL     *string
}

type CardService interface {
	CreateCard(ctx context.Context, p CreateCardParams) (db.GetCardByIDRow, error)
	BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, items []CreateCardParams) ([]db.GetCardByIDRow, error)
	GetCard(ctx context.Context, cardID, userID uuid.UUID) (db.GetCardByIDRow, error)
	ListCardsByDeck(ctx context.Context, deckID, userID uuid.UUID) ([]db.ListCardsByDeckRow, error)
	UpdateCard(ctx context.Context, p UpdateCardParams) (db.GetCardByIDRow, error)
	DeleteCard(ctx context.Context, cardID, userID uuid.UUID) error
}

type cardService struct {
	cardRepo repository.CardRepository
	noteRepo repository.NoteRepository
	deckRepo repository.DeckRepository
}

func NewCardService(
	cardRepo repository.CardRepository,
	noteRepo repository.NoteRepository,
	deckRepo repository.DeckRepository,
) CardService {
	return &cardService{
		cardRepo: cardRepo,
		noteRepo: noteRepo,
		deckRepo: deckRepo,
	}
}

func (s *cardService) CreateCard(ctx context.Context, p CreateCardParams) (db.GetCardByIDRow, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, p.DeckID)
	if err != nil {
		return db.GetCardByIDRow{}, err
	}
	if deck.UserID != p.UserID {
		return db.GetCardByIDRow{}, domain.ErrForbidden
	}

	note, err := s.noteRepo.CreateNote(ctx, db.CreateNoteParams{
		UserID:       p.UserID,
		ContentFront: p.ContentFront,
		ContentBack:  p.ContentBack,
		ImageUrl:     nullStr(p.ImageURL),
	})
	if err != nil {
		return db.GetCardByIDRow{}, err
	}

	card, err := s.cardRepo.CreateCard(ctx, db.CreateCardParams{
		UserID:   p.UserID,
		DeckID:   p.DeckID,
		NoteID:   note.NoteID,
		Position: p.Position,
	})
	if err != nil {
		return db.GetCardByIDRow{}, err
	}

	_ = s.deckRepo.IncrementCardCount(ctx, p.DeckID)

	return db.GetCardByIDRow{
		CardID:       card.CardID,
		UserID:       card.UserID,
		DeckID:       card.DeckID,
		NoteID:       card.NoteID,
		Position:     card.Position,
		CreatedAt:    card.CreatedAt,
		ContentFront: note.ContentFront,
		ContentBack:  note.ContentBack,
		ImageUrl:     note.ImageUrl,
	}, nil
}

func (s *cardService) BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, items []CreateCardParams) ([]db.GetCardByIDRow, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return nil, err
	}
	if deck.UserID != userID {
		return nil, domain.ErrForbidden
	}

	results := make([]db.GetCardByIDRow, 0, len(items))
	for i, item := range items {
		item.UserID = userID
		item.DeckID = deckID
		if item.Position == 0 {
			item.Position = int32(i)
		}
		note, err := s.noteRepo.CreateNote(ctx, db.CreateNoteParams{
			UserID:       userID,
			ContentFront: item.ContentFront,
			ContentBack:  item.ContentBack,
			ImageUrl:     nullStr(item.ImageURL),
		})
		if err != nil {
			return results, err
		}
		card, err := s.cardRepo.CreateCard(ctx, db.CreateCardParams{
			UserID:   userID,
			DeckID:   deckID,
			NoteID:   note.NoteID,
			Position: item.Position,
		})
		if err != nil {
			return results, err
		}
		results = append(results, db.GetCardByIDRow{
			CardID:       card.CardID,
			UserID:       card.UserID,
			DeckID:       card.DeckID,
			NoteID:       card.NoteID,
			Position:     card.Position,
			CreatedAt:    card.CreatedAt,
			ContentFront: note.ContentFront,
			ContentBack:  note.ContentBack,
			ImageUrl:     note.ImageUrl,
		})
	}
	if len(results) > 0 {
		for range results {
			_ = s.deckRepo.IncrementCardCount(ctx, deckID)
		}
	}
	return results, nil
}

func (s *cardService) GetCard(ctx context.Context, cardID, userID uuid.UUID) (db.GetCardByIDRow, error) {
	card, err := s.cardRepo.GetCardByID(ctx, cardID)
	if err != nil {
		return db.GetCardByIDRow{}, err
	}
	deck, err := s.deckRepo.GetDeckByID(ctx, card.DeckID)
	if err != nil {
		return db.GetCardByIDRow{}, err
	}
	if deck.UserID != userID && !deck.IsPublic {
		return db.GetCardByIDRow{}, domain.ErrForbidden
	}
	return card, nil
}

func (s *cardService) ListCardsByDeck(ctx context.Context, deckID, userID uuid.UUID) ([]db.ListCardsByDeckRow, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return nil, err
	}
	if deck.UserID != userID && !deck.IsPublic {
		return nil, domain.ErrForbidden
	}
	return s.cardRepo.ListCardsByDeck(ctx, deckID)
}

func (s *cardService) UpdateCard(ctx context.Context, p UpdateCardParams) (db.GetCardByIDRow, error) {
	card, err := s.cardRepo.GetCardByID(ctx, p.CardID)
	if err != nil {
		return db.GetCardByIDRow{}, err
	}
	if card.UserID != p.UserID {
		return db.GetCardByIDRow{}, domain.ErrForbidden
	}

	updated, err := s.noteRepo.UpdateNote(ctx, db.UpdateNoteParams{
		NoteID:       card.NoteID,
		UserID:       p.UserID,
		ContentFront: nullStr(p.ContentFront),
		ContentBack:  nullStr(p.ContentBack),
		ImageUrl:     nullStr(p.ImageURL),
	})
	if err != nil {
		return db.GetCardByIDRow{}, err
	}

	return db.GetCardByIDRow{
		CardID:       card.CardID,
		UserID:       card.UserID,
		DeckID:       card.DeckID,
		NoteID:       card.NoteID,
		Position:     card.Position,
		CreatedAt:    card.CreatedAt,
		ContentFront: updated.ContentFront,
		ContentBack:  updated.ContentBack,
		ImageUrl:     updated.ImageUrl,
	}, nil
}

func (s *cardService) DeleteCard(ctx context.Context, cardID, userID uuid.UUID) error {
	card, err := s.cardRepo.GetCardByID(ctx, cardID)
	if err != nil {
		return err
	}
	if card.UserID != userID {
		return domain.ErrForbidden
	}
	if err := s.cardRepo.DeleteCard(ctx, db.DeleteCardParams{
		CardID: cardID,
		UserID: userID,
	}); err != nil {
		return err
	}
	_ = s.noteRepo.DeleteNote(ctx, db.DeleteNoteParams{
		NoteID: card.NoteID,
		UserID: userID,
	})
	_ = s.deckRepo.DecrementCardCount(ctx, card.DeckID)
	return nil
}

