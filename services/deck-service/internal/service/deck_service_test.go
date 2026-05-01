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

func makeDeck(id, userID uuid.UUID) db.Deck {
	return db.Deck{
		DeckID:    id,
		UserID:    userID,
		Name:      "Test Deck",
		IsPublic:  false,
		Status:    string(db.ContentStatusActive),
		Settings:  []byte(`{}`),
		CardCount: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ------- CreateDeck -------

func TestCreateDeck_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().CreateDeck(ctx, db.CreateDeckParams{
		UserID:      userID,
		Name:        "Test Deck",
		Description: nullStr(nil),
		IsPublic:    false,
	}).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	result, err := svc.CreateDeck(ctx, CreateDeckParams{UserID: userID, Name: "Test Deck"})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.DeckID != deckID {
		t.Errorf("expected deckID %v, got %v", deckID, result.DeckID)
	}
}

func TestCreateDeck_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	deckRepo.EXPECT().CreateDeck(ctx, gomock.Any()).Return(db.Deck{}, errors.New("db error"))

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.CreateDeck(ctx, CreateDeckParams{UserID: uuid.New(), Name: "Deck"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ------- GetDeck -------

func TestGetDeck_OwnerAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	result, err := svc.GetDeck(ctx, deckID, userID, false)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.DeckID != deckID {
		t.Errorf("expected deckID %v, got %v", deckID, result.DeckID)
	}
}

func TestGetDeck_ForbiddenPrivate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)
	deck.IsPublic = false

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.GetDeck(ctx, deckID, otherID, false)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestGetDeck_PublicAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)
	deck.IsPublic = true

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	result, err := svc.GetDeck(ctx, deckID, otherID, true)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.DeckID != deckID {
		t.Errorf("expected deckID %v, got %v", deckID, result.DeckID)
	}
}

func TestGetDeck_DeletedDeck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)
	deck.Status = string(db.ContentStatusDeleted)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.GetDeck(ctx, deckID, userID, false)

	if !errors.Is(err, domain.ErrDeckNotFound) {
		t.Errorf("expected ErrDeckNotFound, got %v", err)
	}
}

// ------- ListDecks -------

func TestListDecks_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	decks := []db.Deck{makeDeck(uuid.New(), userID), makeDeck(uuid.New(), userID)}

	deckRepo.EXPECT().ListDecksByUser(ctx, db.ListDecksByUserParams{UserID: userID, Limit: 20, Offset: 0}).Return(decks, nil)
	deckRepo.EXPECT().CountDecksByUser(ctx, userID).Return(int64(2), nil)

	svc := NewDeckService(deckRepo, cardRepo)
	page, err := svc.ListDecks(ctx, ListDecksParams{UserID: userID})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(page.Decks) != 2 {
		t.Errorf("expected 2 decks, got %d", len(page.Decks))
	}
	if page.Total != 2 {
		t.Errorf("expected total 2, got %d", page.Total)
	}
}

// ------- ListPublicDecks -------

func TestListPublicDecks_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	decks := []db.Deck{makeDeck(uuid.New(), uuid.New())}

	deckRepo.EXPECT().ListPublicDecks(ctx, db.ListPublicDecksParams{Limit: 20, Offset: 0}).Return(decks, nil)
	deckRepo.EXPECT().CountPublicDecks(ctx).Return(int64(1), nil)

	svc := NewDeckService(deckRepo, cardRepo)
	page, err := svc.ListPublicDecks(ctx, ListPublicDecksParams{})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(page.Decks) != 1 {
		t.Errorf("expected 1 deck, got %d", len(page.Decks))
	}
}

// ------- UpdateDeck -------

func TestUpdateDeck_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)
	newName := "Updated Name"

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	deckRepo.EXPECT().UpdateDeck(ctx, db.UpdateDeckParams{
		DeckID:      deckID,
		UserID:      userID,
		Name:        nullStr(&newName),
		Description: nullStr(nil),
	}).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.UpdateDeck(ctx, UpdateDeckParams{DeckID: deckID, UserID: userID, Name: &newName})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpdateDeck_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.UpdateDeck(ctx, UpdateDeckParams{DeckID: deckID, UserID: otherID})

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- DeleteDeck -------

func TestDeleteDeck_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	deckRepo.EXPECT().SoftDeleteDeck(ctx, db.SoftDeleteDeckParams{DeckID: deckID, UserID: userID}).Return(nil)

	svc := NewDeckService(deckRepo, cardRepo)
	err := svc.DeleteDeck(ctx, deckID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDeleteDeck_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, ownerID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	err := svc.DeleteDeck(ctx, deckID, otherID)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- UpdateSettings -------

func TestUpdateSettings_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	deckRepo.EXPECT().UpdateDeckSettings(ctx, gomock.Any()).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.UpdateSettings(ctx, deckID, userID, DeckSettings{NewCardsPerDay: 20, ReviewsPerDay: 100})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ------- UpdateVisibility -------

func TestUpdateVisibility_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	deckRepo.EXPECT().UpdateDeckVisibility(ctx, db.UpdateDeckVisibilityParams{
		DeckID:   deckID,
		UserID:   userID,
		IsPublic: true,
	}).Return(deck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.UpdateVisibility(ctx, deckID, userID, true)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ------- CloneDeck -------

func TestCloneDeck_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	newOwnerID := uuid.New()
	srcDeckID := uuid.New()
	srcDeck := makeDeck(srcDeckID, ownerID)
	srcDeck.IsPublic = true
	clonedDeck := makeDeck(uuid.New(), newOwnerID)

	deckRepo.EXPECT().GetDeckByID(ctx, srcDeckID).Return(srcDeck, nil)
	deckRepo.EXPECT().CloneDeck(ctx, gomock.Any()).Return(clonedDeck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	result, err := svc.CloneDeck(ctx, srcDeckID, newOwnerID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.UserID != newOwnerID {
		t.Errorf("expected new owner %v, got %v", newOwnerID, result.UserID)
	}
}

func TestCloneDeck_PrivateForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	srcDeckID := uuid.New()
	srcDeck := makeDeck(srcDeckID, ownerID)
	srcDeck.IsPublic = false

	deckRepo.EXPECT().GetDeckByID(ctx, srcDeckID).Return(srcDeck, nil)

	svc := NewDeckService(deckRepo, cardRepo)
	_, err := svc.CloneDeck(ctx, srcDeckID, otherID)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- GetStats -------

func TestGetStats_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deckRepo := mock.NewMockDeckRepository(ctrl)
	cardRepo := mock.NewMockCardRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	deckID := uuid.New()
	deck := makeDeck(deckID, userID)

	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	cardRepo.EXPECT().CountCardsByDeck(ctx, deckID).Return(int64(5), nil)

	svc := NewDeckService(deckRepo, cardRepo)
	stats, err := svc.GetStats(ctx, deckID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.TotalCards != 5 {
		t.Errorf("expected 5 cards, got %d", stats.TotalCards)
	}
}
