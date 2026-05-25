package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"mem_pan/services/admin-service/internal/db"
	"mem_pan/services/admin-service/internal/domain"
)

type AppealRepository interface {
	CreateDeckAppeal(ctx context.Context, arg db.CreateDeckAppealParams) (db.DeckAppeal, error)
	GetDeckAppealByID(ctx context.Context, id uuid.UUID) (db.DeckAppeal, error)
	GetDeckAppealByToken(ctx context.Context, token string) (db.DeckAppeal, error)
	GetDeckAppealByDeck(ctx context.Context, deckID uuid.UUID) (db.DeckAppeal, error)
	ListDeckAppeals(ctx context.Context, arg db.ListDeckAppealsParams) ([]db.DeckAppeal, error)
	CountDeckAppeals(ctx context.Context, statusFilter db.NullAppealStatus) (int64, error)
	SubmitDeckAppeal(ctx context.Context, arg db.SubmitDeckAppealParams) (db.DeckAppeal, error)
	DecideDeckAppeal(ctx context.Context, arg db.DecideDeckAppealParams) (db.DeckAppeal, error)
}

type appealRepository struct {
	q *db.Queries
}

func NewAppealRepository(database *sql.DB) AppealRepository {
	return &appealRepository{q: db.New(database)}
}

func (r *appealRepository) CreateDeckAppeal(ctx context.Context, arg db.CreateDeckAppealParams) (db.DeckAppeal, error) {
	appeal, err := r.q.CreateDeckAppeal(ctx, arg)
	if err != nil {
		// Map unique-violation (deck_id / token already present) to a domain error
		// so callers can treat duplicates as a no-op.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return db.DeckAppeal{}, domain.ErrAppealAlreadyExists
		}
		return db.DeckAppeal{}, err
	}
	return appeal, nil
}

func (r *appealRepository) GetDeckAppealByID(ctx context.Context, id uuid.UUID) (db.DeckAppeal, error) {
	appeal, err := r.q.GetDeckAppealByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.DeckAppeal{}, domain.ErrAppealNotFound
	}
	return appeal, err
}

func (r *appealRepository) GetDeckAppealByToken(ctx context.Context, token string) (db.DeckAppeal, error) {
	appeal, err := r.q.GetDeckAppealByToken(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		return db.DeckAppeal{}, domain.ErrAppealNotFound
	}
	return appeal, err
}

func (r *appealRepository) GetDeckAppealByDeck(ctx context.Context, deckID uuid.UUID) (db.DeckAppeal, error) {
	appeal, err := r.q.GetDeckAppealByDeck(ctx, deckID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.DeckAppeal{}, domain.ErrAppealNotFound
	}
	return appeal, err
}

func (r *appealRepository) ListDeckAppeals(ctx context.Context, arg db.ListDeckAppealsParams) ([]db.DeckAppeal, error) {
	return r.q.ListDeckAppeals(ctx, arg)
}

func (r *appealRepository) CountDeckAppeals(ctx context.Context, statusFilter db.NullAppealStatus) (int64, error) {
	return r.q.CountDeckAppeals(ctx, statusFilter)
}

func (r *appealRepository) SubmitDeckAppeal(ctx context.Context, arg db.SubmitDeckAppealParams) (db.DeckAppeal, error) {
	appeal, err := r.q.SubmitDeckAppeal(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.DeckAppeal{}, domain.ErrAppealNotSubmittable
	}
	return appeal, err
}

func (r *appealRepository) DecideDeckAppeal(ctx context.Context, arg db.DecideDeckAppealParams) (db.DeckAppeal, error) {
	appeal, err := r.q.DecideDeckAppeal(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.DeckAppeal{}, domain.ErrAppealNotSubmittable
	}
	return appeal, err
}
