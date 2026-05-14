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
	pub            publisher.EventPublisher
}

func NewFolderService(
	folderRepo repository.FolderRepository,
	folderDeckRepo repository.FolderDeckRepository,
	deckRepo repository.DeckRepository,
	pubs ...publisher.EventPublisher,
) FolderService {
	var pub publisher.EventPublisher = publisher.NewNoopPublisher()
	if len(pubs) > 0 {
		pub = pubs[0]
	}
	return &folderService{
		folderRepo:     folderRepo,
		folderDeckRepo: folderDeckRepo,
		deckRepo:       deckRepo,
		pub:            pub,
	}
}

func (s *folderService) CreateFolder(ctx context.Context, p CreateFolderParams) (db.Folder, error) {
	folder, err := s.folderRepo.CreateFolder(ctx, db.CreateFolderParams{
		UserID:      p.UserID,
		Name:        p.Name,
		Description: nullStr(p.Description),
	})
	if err != nil {
		return db.Folder{}, err
	}
	if pubErr := s.pub.PublishFolderCreated(ctx, publisher.FolderCreatedEvent{
		FolderID:    folder.FolderID.String(),
		UserID:      folder.UserID.String(),
		Name:        folder.Name,
		Description: nullStrVal(folder.Description),
		CreatedAt:   folder.CreatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] folder.created: %v", pubErr)
	}
	return folder, nil
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
	updated, err := s.folderRepo.UpdateFolder(ctx, db.UpdateFolderParams{
		FolderID:    p.FolderID,
		UserID:      p.UserID,
		Name:        nullStr(p.Name),
		Description: nullStr(p.Description),
	})
	if err != nil {
		return db.Folder{}, err
	}
	if pubErr := s.pub.PublishFolderUpdated(ctx, publisher.FolderUpdatedEvent{
		FolderID:    updated.FolderID.String(),
		UserID:      updated.UserID.String(),
		Name:        updated.Name,
		Description: nullStrVal(updated.Description),
		UpdatedAt:   updated.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] folder.updated: %v", pubErr)
	}
	return updated, nil
}

func (s *folderService) DeleteFolder(ctx context.Context, folderID, userID uuid.UUID) error {
	folder, err := s.folderRepo.GetFolderByID(ctx, folderID)
	if err != nil {
		return err
	}
	if folder.UserID != userID {
		return domain.ErrForbidden
	}
	if err := s.folderRepo.DeleteFolder(ctx, db.DeleteFolderParams{
		FolderID: folderID,
		UserID:   userID,
	}); err != nil {
		return err
	}
	if pubErr := s.pub.PublishFolderDeleted(ctx, publisher.FolderDeletedEvent{
		FolderID: folderID.String(),
		UserID:   userID.String(),
	}); pubErr != nil {
		log.Printf("[publisher] folder.deleted: %v", pubErr)
	}
	return nil
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

