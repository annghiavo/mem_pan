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
	IsPublic    bool
}

type UpdateFolderParams struct {
	FolderID    uuid.UUID
	UserID      uuid.UUID
	Name        *string
	Description *string
}

type ListPublicFoldersParams struct {
	UserID uuid.UUID
	Limit  int32
	Offset int32
}

type FoldersPage struct {
	Folders []db.Folder
	Total   int64
}

type FolderWithDecks struct {
	Folder db.Folder
	Decks  []db.Deck
}

type FolderService interface {
	CreateFolder(ctx context.Context, p CreateFolderParams) (db.Folder, error)
	GetFolder(ctx context.Context, folderID, userID uuid.UUID, publicOK bool) (FolderWithDecks, error)
	ListFolders(ctx context.Context, userID uuid.UUID) ([]db.Folder, error)
	ListPublicFolders(ctx context.Context, p ListPublicFoldersParams) (FoldersPage, error)
	UpdateFolder(ctx context.Context, p UpdateFolderParams) (db.Folder, error)
	UpdateFolderVisibility(ctx context.Context, folderID, userID uuid.UUID, isPublic bool) (db.Folder, error)
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
		IsPublic:    p.IsPublic,
	})
	if err != nil {
		return db.Folder{}, err
	}
	if pubErr := s.pub.PublishFolderCreated(ctx, publisher.FolderCreatedEvent{
		FolderID:    folder.FolderID.String(),
		UserID:      folder.UserID.String(),
		Name:        folder.Name,
		Description: nullStrVal(folder.Description),
		IsPublic:    folder.IsPublic,
		CreatedAt:   folder.CreatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] folder.created: %v", pubErr)
	}
	return folder, nil
}

func (s *folderService) GetFolder(ctx context.Context, folderID, userID uuid.UUID, publicOK bool) (FolderWithDecks, error) {
	folder, err := s.folderRepo.GetFolderByID(ctx, folderID)
	if err != nil {
		return FolderWithDecks{}, err
	}
	isOwner := folder.UserID == userID
	if !isOwner {
		if !publicOK || !folder.IsPublic {
			return FolderWithDecks{}, domain.ErrForbidden
		}
	}
	decks, err := s.folderDeckRepo.ListDecksByFolder(ctx, folderID)
	if err != nil {
		return FolderWithDecks{}, err
	}
	if !isOwner {
		visible := decks[:0]
		for _, d := range decks {
			if d.IsPublic && d.Status == string(db.ContentStatusActive) {
				visible = append(visible, d)
			}
		}
		decks = visible
	}
	return FolderWithDecks{Folder: folder, Decks: decks}, nil
}

func (s *folderService) ListFolders(ctx context.Context, userID uuid.UUID) ([]db.Folder, error) {
	return s.folderRepo.ListFoldersByUser(ctx, userID)
}

func (s *folderService) ListPublicFolders(ctx context.Context, p ListPublicFoldersParams) (FoldersPage, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.UserID != uuid.Nil {
		folders, err := s.folderRepo.ListPublicFoldersByUser(ctx, db.ListPublicFoldersByUserParams{
			UserID: p.UserID,
			Limit:  p.Limit,
			Offset: p.Offset,
		})
		if err != nil {
			return FoldersPage{}, err
		}
		total, err := s.folderRepo.CountPublicFoldersByUser(ctx, p.UserID)
		if err != nil {
			return FoldersPage{}, err
		}
		return FoldersPage{Folders: folders, Total: total}, nil
	}
	folders, err := s.folderRepo.ListPublicFolders(ctx, db.ListPublicFoldersParams{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return FoldersPage{}, err
	}
	total, err := s.folderRepo.CountPublicFolders(ctx)
	if err != nil {
		return FoldersPage{}, err
	}
	return FoldersPage{Folders: folders, Total: total}, nil
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
		IsPublic:    updated.IsPublic,
		UpdatedAt:   updated.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] folder.updated: %v", pubErr)
	}
	return updated, nil
}

func (s *folderService) UpdateFolderVisibility(ctx context.Context, folderID, userID uuid.UUID, isPublic bool) (db.Folder, error) {
	folder, err := s.folderRepo.GetFolderByID(ctx, folderID)
	if err != nil {
		return db.Folder{}, err
	}
	if folder.UserID != userID {
		return db.Folder{}, domain.ErrForbidden
	}
	updated, err := s.folderRepo.UpdateFolderVisibility(ctx, db.UpdateFolderVisibilityParams{
		FolderID: folderID,
		UserID:   userID,
		IsPublic: isPublic,
	})
	if err != nil {
		return db.Folder{}, err
	}
	if pubErr := s.pub.PublishFolderUpdated(ctx, publisher.FolderUpdatedEvent{
		FolderID:    updated.FolderID.String(),
		UserID:      updated.UserID.String(),
		Name:        updated.Name,
		Description: nullStrVal(updated.Description),
		IsPublic:    updated.IsPublic,
		UpdatedAt:   updated.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] folder.updated (visibility): %v", pubErr)
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
