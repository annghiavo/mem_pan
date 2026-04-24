package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
)

type NoteRepository interface {
	CreateNote(ctx context.Context, arg db.CreateNoteParams) (db.Note, error)
	GetNoteByID(ctx context.Context, id uuid.UUID) (db.Note, error)
	UpdateNote(ctx context.Context, arg db.UpdateNoteParams) (db.Note, error)
	DeleteNote(ctx context.Context, arg db.DeleteNoteParams) error
}

type noteRepository struct {
	q *db.Queries
}

func NewNoteRepository(database *sql.DB) NoteRepository {
	return &noteRepository{q: db.New(database)}
}

func (r *noteRepository) CreateNote(ctx context.Context, arg db.CreateNoteParams) (db.Note, error) {
	return r.q.CreateNote(ctx, arg)
}

func (r *noteRepository) GetNoteByID(ctx context.Context, id uuid.UUID) (db.Note, error) {
	n, err := r.q.GetNoteByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Note{}, domain.ErrNoteNotFound
	}
	return n, err
}

func (r *noteRepository) UpdateNote(ctx context.Context, arg db.UpdateNoteParams) (db.Note, error) {
	n, err := r.q.UpdateNote(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Note{}, domain.ErrNoteNotFound
	}
	return n, err
}

func (r *noteRepository) DeleteNote(ctx context.Context, arg db.DeleteNoteParams) error {
	return r.q.DeleteNote(ctx, arg)
}
