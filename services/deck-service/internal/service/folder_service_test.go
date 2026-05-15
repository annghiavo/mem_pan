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

func makeFolder(id, userID uuid.UUID) db.Folder {
	return db.Folder{
		FolderID:  id,
		UserID:    userID,
		Name:      "Test Folder",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ------- CreateFolder -------

func TestCreateFolder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folderID := uuid.New()
	folder := makeFolder(folderID, userID)

	folderRepo.EXPECT().CreateFolder(ctx, db.CreateFolderParams{
		UserID:      userID,
		Name:        "Test Folder",
		Description: nullStr(nil),
	}).Return(folder, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	result, err := svc.CreateFolder(ctx, CreateFolderParams{UserID: userID, Name: "Test Folder"})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.FolderID != folderID {
		t.Errorf("expected folderID %v, got %v", folderID, result.FolderID)
	}
}

func TestCreateFolder_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	folderRepo.EXPECT().CreateFolder(ctx, gomock.Any()).Return(db.Folder{}, errors.New("db error"))

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	_, err := svc.CreateFolder(ctx, CreateFolderParams{UserID: uuid.New(), Name: "Folder"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ------- GetFolder -------

func TestGetFolder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folderID := uuid.New()
	folder := makeFolder(folderID, userID)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)
	folderDeckRepo.EXPECT().ListDecksByFolder(ctx, folderID).Return([]db.Deck{}, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	result, err := svc.GetFolder(ctx, folderID, userID, false)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Folder.FolderID != folderID {
		t.Errorf("expected folderID %v, got %v", folderID, result.Folder.FolderID)
	}
}

func TestGetFolder_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	folderID := uuid.New()
	folder := makeFolder(folderID, ownerID)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	_, err := svc.GetFolder(ctx, folderID, otherID, false)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- ListFolders -------

func TestListFolders_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folders := []db.Folder{makeFolder(uuid.New(), userID), makeFolder(uuid.New(), userID)}

	folderRepo.EXPECT().ListFoldersByUser(ctx, userID).Return(folders, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	result, err := svc.ListFolders(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 folders, got %d", len(result))
	}
}

// ------- UpdateFolder -------

func TestUpdateFolder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folderID := uuid.New()
	folder := makeFolder(folderID, userID)
	newName := "Updated"

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)
	folderRepo.EXPECT().UpdateFolder(ctx, db.UpdateFolderParams{
		FolderID:    folderID,
		UserID:      userID,
		Name:        nullStr(&newName),
		Description: nullStr(nil),
	}).Return(folder, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	_, err := svc.UpdateFolder(ctx, UpdateFolderParams{FolderID: folderID, UserID: userID, Name: &newName})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpdateFolder_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	folderID := uuid.New()
	folder := makeFolder(folderID, ownerID)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	_, err := svc.UpdateFolder(ctx, UpdateFolderParams{FolderID: folderID, UserID: otherID})

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- DeleteFolder -------

func TestDeleteFolder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folderID := uuid.New()
	folder := makeFolder(folderID, userID)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)
	folderRepo.EXPECT().DeleteFolder(ctx, db.DeleteFolderParams{FolderID: folderID, UserID: userID}).Return(nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	err := svc.DeleteFolder(ctx, folderID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDeleteFolder_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	ownerID := uuid.New()
	otherID := uuid.New()
	folderID := uuid.New()
	folder := makeFolder(folderID, ownerID)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	err := svc.DeleteFolder(ctx, folderID, otherID)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- AddDeckToFolder -------

func TestAddDeckToFolder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folderID := uuid.New()
	deckID := uuid.New()
	folder := makeFolder(folderID, userID)
	deck := makeDeck(deckID, userID)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)
	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)
	folderDeckRepo.EXPECT().AddDeckToFolder(ctx, db.AddDeckToFolderParams{FolderID: folderID, DeckID: deckID}).Return(db.FolderDeck{}, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	err := svc.AddDeckToFolder(ctx, folderID, deckID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDeckToFolder_DeletedDeck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folderID := uuid.New()
	deckID := uuid.New()
	folder := makeFolder(folderID, userID)
	deck := makeDeck(deckID, userID)
	deck.Status = string(db.ContentStatusDeleted)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)
	deckRepo.EXPECT().GetDeckByID(ctx, deckID).Return(deck, nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	err := svc.AddDeckToFolder(ctx, folderID, deckID, userID)

	if !errors.Is(err, domain.ErrDeckDeleted) {
		t.Errorf("expected ErrDeckDeleted, got %v", err)
	}
}

// ------- RemoveDeckFromFolder -------

func TestRemoveDeckFromFolder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	folderRepo := mock.NewMockFolderRepository(ctrl)
	folderDeckRepo := mock.NewMockFolderDeckRepository(ctrl)
	deckRepo := mock.NewMockDeckRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	folderID := uuid.New()
	deckID := uuid.New()
	folder := makeFolder(folderID, userID)

	folderRepo.EXPECT().GetFolderByID(ctx, folderID).Return(folder, nil)
	folderDeckRepo.EXPECT().RemoveDeckFromFolder(ctx, db.RemoveDeckFromFolderParams{FolderID: folderID, DeckID: deckID}).Return(nil)

	svc := NewFolderService(folderRepo, folderDeckRepo, deckRepo)
	err := svc.RemoveDeckFromFolder(ctx, folderID, deckID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
