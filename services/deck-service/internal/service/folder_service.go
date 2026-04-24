package service

import (
	"context"
	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/repository"
)

type CreateFolderParams struct {
	UserID      uuid.UUID
	Name        string
	Description *string
}

type UpdateFolderParams struct {
	FolderID    uuid.UUID
	UserID      uuid.UUID
	Name        *string
	Description *string
}

type FolderWithDecks struct {
	Folder db.Folder
	Decks  []db.Deck
}

type FolderService interface {
	CreateFolder(ctx context.Context, p CreateFolderParams) (db.Folder, error)
	GetFolder(ctx context.Context, folderID, userID uuid.UUID) (FolderWithDecks, error)
	ListFolders(ctx context.Context, userID uuid.UUID) ([]db.Folder, error)
	UpdateFolder(ctx context.Context, p UpdateFolderParams) (db.Folder, error)
	DeleteFolder(ctx context.Context, folderID, userID uuid.UUID) error
	AddDeckToFolder(ctx context.Context, folderID, deckID, userID uuid.UUID) error
	RemoveDeckFromFolder(ctx context.Context, folderID, deckID, userID uuid.UUID) error
}

type folderService struct {
	folderRepo     repository.FolderRepository
	folderDeckRepo repository.FolderDeckRepository
	deckRepo       repository.DeckRepository
}

func NewFolderService(
	folderRepo repository.FolderRepository,
	folderDeckRepo repository.FolderDeckRepository,
	deckRepo repository.DeckRepository,
) FolderService {
	return &folderService{
		folderRepo:     folderRepo,
		folderDeckRepo: folderDeckRepo,
		deckRepo:       deckRepo,
	}
}

func (s *folderService) CreateFolder(ctx context.Context, p CreateFolderParams) (db.Folder, error) {
	return s.folderRepo.CreateFolder(ctx, db.CreateFolderParams{
		UserID:      p.UserID,
		Name:        p.Name,
		Description: nullStr(p.Description),
	})
}

func (s *folderService) GetFolder(ctx context.Context, folderID, userID uuid.UUID) (FolderWithDecks, error) {
	folder, err := s.folderRepo.GetFolderByID(ctx, folderID)
	if err != nil {
		return FolderWithDecks{}, err
	}
	if folder.UserID != userID {
		return FolderWithDecks{}, domain.ErrForbidden
	}
	decks, err := s.folderDeckRepo.ListDecksByFolder(ctx, folderID)
	if err != nil {
		return FolderWithDecks{}, err
	}
	return FolderWithDecks{Folder: folder, Decks: decks}, nil
}

func (s *folderService) ListFolders(ctx context.Context, userID uuid.UUID) ([]db.Folder, error) {
	return s.folderRepo.ListFoldersByUser(ctx, userID)
}

func (s *folderService) UpdateFolder(ctx context.Context, p UpdateFolderParams) (db.Folder, error) {
	folder, err := s.folderRepo.GetFolderByID(ctx, p.FolderID)
	if err != nil {
		return db.Folder{}, err
	}
	if folder.UserID != p.UserID {
		return db.Folder{}, domain.ErrForbidden
	}
	return s.folderRepo.UpdateFolder(ctx, db.UpdateFolderParams{
		FolderID:    p.FolderID,
		UserID:      p.UserID,
		Name:        nullStr(p.Name),
		Description: nullStr(p.Description),
	})
}

func (s *folderService) DeleteFolder(ctx context.Context, folderID, userID uuid.UUID) error {
	folder, err := s.folderRepo.GetFolderByID(ctx, folderID)
	if err != nil {
		return err
	}
	if folder.UserID != userID {
		return domain.ErrForbidden
	}
	return s.folderRepo.DeleteFolder(ctx, db.DeleteFolderParams{
		FolderID: folderID,
		UserID:   userID,
	})
}

func (s *folderService) AddDeckToFolder(ctx context.Context, folderID, deckID, userID uuid.UUID) error {
	folder, err := s.folderRepo.GetFolderByID(ctx, folderID)
	if err != nil {
		return err
	}
	if folder.UserID != userID {
		return domain.ErrForbidden
	}
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return err
	}
	if deck.Status == string(db.ContentStatusDeleted) {
		return domain.ErrDeckDeleted
	}
	_, err = s.folderDeckRepo.AddDeckToFolder(ctx, db.AddDeckToFolderParams{
		FolderID: folderID,
		DeckID:   deckID,
	})
	return err
}

func (s *folderService) RemoveDeckFromFolder(ctx context.Context, folderID, deckID, userID uuid.UUID) error {
	folder, err := s.folderRepo.GetFolderByID(ctx, folderID)
	if err != nil {
		return err
	}
	if folder.UserID != userID {
		return domain.ErrForbidden
	}
	return s.folderDeckRepo.RemoveDeckFromFolder(ctx, db.RemoveDeckFromFolderParams{
		FolderID: folderID,
		DeckID:   deckID,
	})
}

