package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
)

type FolderDeckRepository interface {
	AddDeckToFolder(ctx context.Context, arg db.AddDeckToFolderParams) (db.FolderDeck, error)
	RemoveDeckFromFolder(ctx context.Context, arg db.RemoveDeckFromFolderParams) error
	ListDecksByFolder(ctx context.Context, folderID uuid.UUID) ([]db.Deck, error)
	IsDeckInFolder(ctx context.Context, arg db.IsDeckInFolderParams) (bool, error)
}

type folderDeckRepository struct {
	q *db.Queries
}

func NewFolderDeckRepository(database *sql.DB) FolderDeckRepository {
	return &folderDeckRepository{q: db.New(database)}
}

func (r *folderDeckRepository) AddDeckToFolder(ctx context.Context, arg db.AddDeckToFolderParams) (db.FolderDeck, error) {
	fd, err := r.q.AddDeckToFolder(ctx, arg)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return db.FolderDeck{}, domain.ErrDeckAlreadyInFolder
		}
		return db.FolderDeck{}, err
	}
	return fd, nil
}

func (r *folderDeckRepository) RemoveDeckFromFolder(ctx context.Context, arg db.RemoveDeckFromFolderParams) error {
	return r.q.RemoveDeckFromFolder(ctx, arg)
}

func (r *folderDeckRepository) ListDecksByFolder(ctx context.Context, folderID uuid.UUID) ([]db.Deck, error) {
	return r.q.ListDecksByFolder(ctx, folderID)
}

func (r *folderDeckRepository) IsDeckInFolder(ctx context.Context, arg db.IsDeckInFolderParams) (bool, error) {
	return r.q.IsDeckInFolder(ctx, arg)
}
