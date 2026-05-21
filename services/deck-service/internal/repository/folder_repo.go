package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
)

type FolderRepository interface {
	CreateFolder(ctx context.Context, arg db.CreateFolderParams) (db.Folder, error)
	GetFolderByID(ctx context.Context, id uuid.UUID) (db.Folder, error)
	ListFoldersByUser(ctx context.Context, userID uuid.UUID) ([]db.Folder, error)
	ListPublicFolders(ctx context.Context, arg db.ListPublicFoldersParams) ([]db.Folder, error)
	CountPublicFolders(ctx context.Context) (int64, error)
	UpdateFolder(ctx context.Context, arg db.UpdateFolderParams) (db.Folder, error)
	UpdateFolderVisibility(ctx context.Context, arg db.UpdateFolderVisibilityParams) (db.Folder, error)
	DeleteFolder(ctx context.Context, arg db.DeleteFolderParams) error
}

type folderRepository struct {
	q *db.Queries
}

func NewFolderRepository(database *sql.DB) FolderRepository {
	return &folderRepository{q: db.New(database)}
}

func (r *folderRepository) CreateFolder(ctx context.Context, arg db.CreateFolderParams) (db.Folder, error) {
	f, err := r.q.CreateFolder(ctx, arg)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return db.Folder{}, err
		}
		return db.Folder{}, err
	}
	return f, nil
}

func (r *folderRepository) GetFolderByID(ctx context.Context, id uuid.UUID) (db.Folder, error) {
	f, err := r.q.GetFolderByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Folder{}, domain.ErrFolderNotFound
	}
	return f, err
}

func (r *folderRepository) ListFoldersByUser(ctx context.Context, userID uuid.UUID) ([]db.Folder, error) {
	return r.q.ListFoldersByUser(ctx, userID)
}

func (r *folderRepository) ListPublicFolders(ctx context.Context, arg db.ListPublicFoldersParams) ([]db.Folder, error) {
	return r.q.ListPublicFolders(ctx, arg)
}

func (r *folderRepository) CountPublicFolders(ctx context.Context) (int64, error) {
	return r.q.CountPublicFolders(ctx)
}

func (r *folderRepository) UpdateFolder(ctx context.Context, arg db.UpdateFolderParams) (db.Folder, error) {
	f, err := r.q.UpdateFolder(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Folder{}, domain.ErrFolderNotFound
	}
	return f, err
}

func (r *folderRepository) UpdateFolderVisibility(ctx context.Context, arg db.UpdateFolderVisibilityParams) (db.Folder, error) {
	f, err := r.q.UpdateFolderVisibility(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Folder{}, domain.ErrFolderNotFound
	}
	return f, err
}

func (r *folderRepository) DeleteFolder(ctx context.Context, arg db.DeleteFolderParams) error {
	return r.q.DeleteFolder(ctx, arg)
}
