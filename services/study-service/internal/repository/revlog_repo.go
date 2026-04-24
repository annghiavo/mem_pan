package repository

import (
	"context"
	"database/sql"

	"mem_pan/services/study-service/internal/db"
)

type RevlogRepository interface {
	InsertRevlog(ctx context.Context, arg db.InsertRevlogParams) (db.Revlog, error)
}

type revlogRepository struct {
	q *db.Queries
}

func NewRevlogRepository(database *sql.DB) RevlogRepository {
	return &revlogRepository{q: db.New(database)}
}

func (r *revlogRepository) InsertRevlog(ctx context.Context, arg db.InsertRevlogParams) (db.Revlog, error) {
	return r.q.InsertRevlog(ctx, arg)
}
