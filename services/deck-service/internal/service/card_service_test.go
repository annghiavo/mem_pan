package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/mock"
)

func makeNote(id, userID uuid.UUID) db.Note {
	return db.Note{
		NoteID:       id,
		UserID:       userID,
		ContentFront: "Front",
		ContentBack:  "Back",
		LangFront:    "en",
		LangBack:     "en",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func makeCard(cardID, noteID, deckID, userID uuid.UUID) db.Card {
	return db.Card{
		CardID:    cardID,
		UserID:    userID,
		DeckID:    deckID,
		NoteID:    noteID,
		Position:  0,
		CreatedAt: time.Now(),
	}
}

func makeCardRow(cardID, noteID, deckID, userID uuid.UUID) db.GetCardByIDRow {
	return db.GetCardByIDRow{
		CardID:       cardID,
		UserID:       userID,
		DeckID:       deckID,
		NoteID:       noteID,
		Position:     0,
		ContentFront: "Front",
		ContentBack:  "Back",
		LangFront:    "en",
		LangBack:     "en",
		CreatedAt:    time.Now(),
	}
}

// ------- CreateCard -------

func TestCreateCard_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	noteID := uuid.New()
	deck := makeDeck(deckID, userID)
	note := makeNote(noteID, userID)
	card := makeCard(cardID, noteID, deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	noteRepo.EXPECT().CreateNote(ctx, gomock.Any()).Return(note, nil)
	cardRepo.EXPECT().CreateCard(ctx, gomock.Any()).Return(card, nil)
	deckRepo.EXPECT().IncrementCardCount(ctx, deckID).Return(nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	result, err := svc.CreateCard(ctx, CreateCardParams{
		UserID:       userID,
		DeckID:       deckID,
		ContentFront: "Front",
		ContentBack:  "Back",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CardID != cardID {
		t.Errorf("expected cardID %v, got %v", cardID, result.CardID)
	}
}

func TestCreateCard_DeckForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	_, err := svc.CreateCard(ctx, CreateCardParams{
		UserID:       otherID,
		DeckID:       deckID,
		ContentFront: "Front",
		ContentBack:  "Back",
	})

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestCreateCard_NoteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	noteRepo.EXPECT().CreateNote(ctx, gomock.Any()).Return(db.Note{}, errors.New("db error"))

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	_, err := svc.CreateCard(ctx, CreateCardParams{
		UserID:       userID,
		DeckID:       deckID,
		ContentFront: "Front",
		ContentBack:  "Back",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ------- BulkCreateCards -------

func TestBulkCreateCards_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	noteRepo.EXPECT().CreateNote(ctx, gomock.Any()).Return(makeNote(uuid.New(), userID), nil).Times(2)
	cardRepo.EXPECT().CreateCard(ctx, gomock.Any()).Return(makeCard(uuid.New(), uuid.New(), deckID, userID), nil).Times(2)
	deckRepo.EXPECT().IncrementCardCount(ctx, deckID).Return(nil).Times(2)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	items := []CreateCardParams{
		{ContentFront: "Front1", ContentBack: "Back1"},
		{ContentFront: "Front2", ContentBack: "Back2"},
	}
	results, err := svc.BulkCreateCards(ctx, userID, deckID, items)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 cards, got %d", len(results))
	}
}

func TestBulkCreateCards_DeckForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	_, err := svc.BulkCreateCards(ctx, otherID, deckID, []CreateCardParams{{ContentFront: "F", ContentBack: "B"}})

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- GetCard -------

func TestGetCard_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	noteID := uuid.New()
	deck := makeDeck(deckID, userID)
	cardRow := makeCardRow(cardID, noteID, deckID, userID)

	cardRepo.EXPECT().GetCardByID(ctx, cardID).Return(cardRow, nil)
	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	result, err := svc.GetCard(ctx, cardID, userID, false)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CardID != cardID {
		t.Errorf("expected cardID %v, got %v", cardID, result.CardID)
	}
}

func TestGetCard_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	cardID := uuid.New()
	cardRepo.EXPECT().GetCardByID(ctx, cardID).Return(db.GetCardByIDRow{}, domain.ErrCardNotFound)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	_, err := svc.GetCard(ctx, cardID, uuid.New(), false)

	if !errors.Is(err, domain.ErrCardNotFound) {
		t.Errorf("expected ErrCardNotFound, got %v", err)
	}
}

// ------- ListCardsByDeck -------

func TestListCardsByDeck_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	cardRepo.EXPECT().ListCardsByDeck(ctx, deckID).Return([]db.ListCardsByDeckRow{}, nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	_, err := svc.ListCardsByDeck(ctx, deckID, userID, false)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestListCardsByDeck_PlusPreviewForNonSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)
	deck.IsPublic = true
	deck.AccessLevel = db.DeckAccessLevelPlus
	deck.PlusStatus = db.DeckPlusStatusApproved
	deck.CardCount = 208

	cards := make([]db.ListCardsByDeckRow, 30)
	for i := range cards {
		cards[i] = db.ListCardsByDeckRow{CardID: uuid.New(), DeckID: deckID, UserID: ownerID, Position: int32(i), ContentFront: "Front", ContentBack: "Back", LangFront: "en", LangBack: "en"}
	}

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	cardRepo.EXPECT().ListCardsByDeck(ctx, deckID).Return(cards, nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	result, err := svc.ListCardsByDeck(ctx, deckID, otherID, false)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 21 {
		t.Fatalf("expected 21 preview cards, got %d", len(result))
	}
}

func TestListCardsByDeck_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)
	deck.IsPublic = false

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	_, err := svc.ListCardsByDeck(ctx, deckID, otherID, false)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- DeleteCard -------

func TestDeleteCard_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	noteID := uuid.New()
	cardRow := makeCardRow(cardID, noteID, deckID, userID)

	cardRepo.EXPECT().GetCardByID(ctx, cardID).Return(cardRow, nil)
	cardRepo.EXPECT().DeleteCard(ctx, db.DeleteCardParams{CardID: cardID, UserID: userID}).Return(nil)
	noteRepo.EXPECT().DeleteNote(ctx, db.DeleteNoteParams{NoteID: noteID, UserID: userID}).Return(nil)
	deckRepo.EXPECT().DecrementCardCount(ctx, deckID).Return(nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	err := svc.DeleteCard(ctx, cardID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDeleteCard_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	noteID := uuid.New()
	cardRow := makeCardRow(cardID, noteID, deckID, ownerID)

	cardRepo.EXPECT().GetCardByID(ctx, cardID).Return(cardRow, nil)

	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	err := svc.DeleteCard(ctx, cardID, otherID)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- UpdateCard -------

func TestUpdateCard_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	noteRepo := mock.NewMockNoteRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	noteID := uuid.New()
	cardRow := makeCardRow(cardID, noteID, deckID, userID)
	note := makeNote(noteID, userID)

	cardRepo.EXPECT().GetCardByID(ctx, cardID).Return(cardRow, nil)
	noteRepo.EXPECT().UpdateNote(ctx, gomock.Any()).Return(note, nil)

	newFront := "New Front"
	svc := NewCardService(cardRepo, noteRepo, deckRepo)
	_, err := svc.UpdateCard(ctx, UpdateCardParams{
		CardID:       cardID,
		UserID:       userID,
		ContentFront: &newFront,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
