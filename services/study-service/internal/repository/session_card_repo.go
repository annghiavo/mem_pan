package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/domain"
)

type SessionCardRepository interface {
	InsertSessionCard(ctx context.Context, arg db.InsertSessionCardParams) (db.SessionCard, error)
	ListSessionCards(ctx context.Context, sessionID uuid.UUID) ([]db.SessionCard, error)
	MarkSessionCardReviewed(ctx context.Context, arg db.MarkSessionCardReviewedParams) (db.SessionCard, error)
	GetSessionCardByCard(ctx context.Context, arg db.GetSessionCardByCardParams) (db.SessionCard, error)
}

type sessionCardRepository struct {
	q *db.Queries
}

func NewSessionCardRepository(database *sql.DB) SessionCardRepository {
	return &sessionCardRepository{q: db.New(database)}
}

func (r *sessionCardRepository) InsertSessionCard(ctx context.Context, arg db.InsertSessionCardParams) (db.SessionCard, error) {
	return r.q.InsertSessionCard(ctx, arg)
}

func (r *sessionCardRepository) ListSessionCards(ctx context.Context, sessionID uuid.UUID) ([]db.SessionCard, error) {
	return r.q.ListSessionCards(ctx, sessionID)
}

func (r *sessionCardRepository) MarkSessionCardReviewed(ctx context.Context, arg db.MarkSessionCardReviewedParams) (db.SessionCard, error) {
	sc, err := r.q.MarkSessionCardReviewed(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.SessionCard{}, domain.ErrCardNotInSession
	}
	return sc, err
}

func (r *sessionCardRepository) GetSessionCardByCard(ctx context.Context, arg db.GetSessionCardByCardParams) (db.SessionCard, error) {
	sc, err := r.q.GetSessionCardByCard(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.SessionCard{}, domain.ErrCardNotInSession
	}
	return sc, err
}
