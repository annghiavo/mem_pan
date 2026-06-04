package service

import (
	"context"
	"log"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/publisher"
	"mem_pan/services/deck-service/internal/repository"
)

type CreateCardParams struct {
	UserID       uuid.UUID
	DeckID       uuid.UUID
	ContentFront string
	ContentBack  string
	ImageURL     *string
	Position     int32
	LangFront    string
	LangBack     string
}

type UpdateCardParams struct {
	CardID       uuid.UUID
	UserID       uuid.UUID
	ContentFront *string
	ContentBack  *string
	ImageURL     *string
	LangFront    *string
	LangBack     *string
}

type CardService interface {
	CreateCard(ctx context.Context, p CreateCardParams) (db.GetCardByIDRow, error)
	BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, items []CreateCardParams) ([]db.GetCardByIDRow, error)
	GetCard(ctx context.Context, cardID, userID uuid.UUID) (db.GetCardByIDRow, error)
	ListCardsByDeck(ctx context.Context, deckID, userID uuid.UUID) ([]db.ListCardsByDeckRow, error)
	UpdateCard(ctx context.Context, p UpdateCardParams) (db.GetCardByIDRow, error)
	DeleteCard(ctx context.Context, cardID, userID uuid.UUID) error
	// ReorderCards updates the position of each card in the provided ordered list.
	// cardIDs must contain only card IDs belonging to the specified deck.
	ReorderCards(ctx context.Context, deckID, userID uuid.UUID, cardIDs []uuid.UUID) error
}

type cardService struct {
	cardRepo repository.CardRepository
	noteRepo repository.NoteRepository
	deckRepo repository.DeckRepository
	pub      publisher.EventPublisher
}

func NewCardService(
	cardRepo repository.CardRepository,
	noteRepo repository.NoteRepository,
	deckRepo repository.DeckRepository,
	pubs ...publisher.EventPublisher,
) CardService {
	var pub publisher.EventPublisher = publisher.NewNoopPublisher()
	if len(pubs) > 0 {
		pub = pubs[0]
	}
	return &cardService{
		cardRepo: cardRepo,
		noteRepo: noteRepo,
		deckRepo: deckRepo,
		pub:      pub,
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

	langFront := p.LangFront
	if langFront == "" {
		langFront = "en"
	}
	langBack := p.LangBack
	if langBack == "" {
		langBack = "en"
	}

	note, err := s.noteRepo.CreateNote(ctx, db.CreateNoteParams{
		UserID:       p.UserID,
		ContentFront: p.ContentFront,
		ContentBack:  p.ContentBack,
		ImageUrl:     nullStr(p.ImageURL),
		LangFront:    langFront,
		LangBack:     langBack,
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

	result := db.GetCardByIDRow{
		CardID:       card.CardID,
		UserID:       card.UserID,
		DeckID:       card.DeckID,
		NoteID:       card.NoteID,
		Position:     card.Position,
		CreatedAt:    card.CreatedAt,
		ContentFront: note.ContentFront,
		ContentBack:  note.ContentBack,
		ImageUrl:     note.ImageUrl,
		LangFront:    note.LangFront,
		LangBack:     note.LangBack,
	}
	if pubErr := s.pub.PublishCardCreated(ctx, publisher.CardCreatedEvent{
		CardID:       card.CardID.String(),
		DeckID:       card.DeckID.String(),
		UserID:       card.UserID.String(),
		NoteID:       card.NoteID.String(),
		ContentFront: note.ContentFront,
		ContentBack:  note.ContentBack,
		ImageURL:     note.ImageUrl.String,
		CreatedAt:    card.CreatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] card.created: %v", pubErr)
	}
	return result, nil
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
		lf := item.LangFront
		if lf == "" {
			lf = "en"
		}
		lb := item.LangBack
		if lb == "" {
			lb = "en"
		}
		note, err := s.noteRepo.CreateNote(ctx, db.CreateNoteParams{
			UserID:       userID,
			ContentFront: item.ContentFront,
			ContentBack:  item.ContentBack,
			ImageUrl:     nullStr(item.ImageURL),
			LangFront:    lf,
			LangBack:     lb,
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
			LangFront:    note.LangFront,
			LangBack:     note.LangBack,
		})
		if pubErr := s.pub.PublishCardCreated(ctx, publisher.CardCreatedEvent{
			CardID:       card.CardID.String(),
			DeckID:       card.DeckID.String(),
			UserID:       card.UserID.String(),
			NoteID:       card.NoteID.String(),
			ContentFront: note.ContentFront,
			ContentBack:  note.ContentBack,
			ImageURL:     note.ImageUrl.String,
			CreatedAt:    card.CreatedAt,
		}); pubErr != nil {
			log.Printf("[publisher] card.created: %v", pubErr)
		}
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
		LangFront:    nullLang(p.LangFront),
		LangBack:     nullLang(p.LangBack),
	})
	if err != nil {
		return db.GetCardByIDRow{}, err
	}

	result := db.GetCardByIDRow{
		CardID:       card.CardID,
		UserID:       card.UserID,
		DeckID:       card.DeckID,
		NoteID:       card.NoteID,
		Position:     card.Position,
		CreatedAt:    card.CreatedAt,
		ContentFront: updated.ContentFront,
		ContentBack:  updated.ContentBack,
		ImageUrl:     updated.ImageUrl,
		LangFront:    updated.LangFront,
		LangBack:     updated.LangBack,
	}
	if pubErr := s.pub.PublishCardUpdated(ctx, publisher.CardUpdatedEvent{
		CardID:       card.CardID.String(),
		DeckID:       card.DeckID.String(),
		UserID:       card.UserID.String(),
		NoteID:       card.NoteID.String(),
		ContentFront: updated.ContentFront,
		ContentBack:  updated.ContentBack,
		ImageURL:     updated.ImageUrl.String,
	}); pubErr != nil {
		log.Printf("[publisher] card.updated: %v", pubErr)
	}
	return result, nil
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
	if pubErr := s.pub.PublishCardDeleted(ctx, publisher.CardDeletedEvent{
		CardID: cardID.String(),
		DeckID: card.DeckID.String(),
		UserID: userID.String(),
	}); pubErr != nil {
		log.Printf("[publisher] card.deleted: %v", pubErr)
	}
	return nil
}

func (s *cardService) ReorderCards(ctx context.Context, deckID, userID uuid.UUID, cardIDs []uuid.UUID) error {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return err
	}
	if deck.UserID != userID {
		return domain.ErrForbidden
	}
	for i, cid := range cardIDs {
		if err := s.cardRepo.UpdateCardPosition(ctx, db.UpdateCardPositionParams{
			Position: int32(i),
			CardID:   cid,
			UserID:   userID,
		}); err != nil {
			return err
		}
	}
	return nil
}
