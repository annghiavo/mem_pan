package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/domain"
)

type StudySessionRepository interface {
	CreateStudySession(ctx context.Context, arg db.CreateStudySessionParams) (db.StudySession, error)
	GetStudySession(ctx context.Context, id uuid.UUID) (db.StudySession, error)
	GetOngoingSessionByDeck(ctx context.Context, arg db.GetOngoingSessionByDeckParams) (db.StudySession, error)
	FinishStudySession(ctx context.Context, id uuid.UUID) (db.StudySession, error)
	AbandonStudySession(ctx context.Context, id uuid.UUID) (db.StudySession, error)
	IncrementCompletedCards(ctx context.Context, id uuid.UUID) (db.StudySession, error)
	GetMostRecentSession(ctx context.Context, userID uuid.UUID) (db.StudySession, error)
	ListRecentDecks(ctx context.Context, userID uuid.UUID) ([]db.ListRecentDecksRow, error)
}

type studySessionRepository struct {
	q *db.Queries
}

func NewStudySessionRepository(database *sql.DB) StudySessionRepository {
	return &studySessionRepository{q: db.New(database)}
}

func (r *studySessionRepository) CreateStudySession(ctx context.Context, arg db.CreateStudySessionParams) (db.StudySession, error) {
	return r.q.CreateStudySession(ctx, arg)
}

func (r *studySessionRepository) GetStudySession(ctx context.Context, id uuid.UUID) (db.StudySession, error) {
	s, err := r.q.GetStudySession(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.StudySession{}, domain.ErrSessionNotFound
	}
	return s, err
}

func (r *studySessionRepository) GetOngoingSessionByDeck(ctx context.Context, arg db.GetOngoingSessionByDeckParams) (db.StudySession, error) {
	s, err := r.q.GetOngoingSessionByDeck(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.StudySession{}, domain.ErrSessionNotFound
	}
	return s, err
}

func (r *studySessionRepository) FinishStudySession(ctx context.Context, id uuid.UUID) (db.StudySession, error) {
	s, err := r.q.FinishStudySession(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.StudySession{}, domain.ErrSessionNotFound
	}
	return s, err
}

func (r *studySessionRepository) AbandonStudySession(ctx context.Context, id uuid.UUID) (db.StudySession, error) {
	s, err := r.q.AbandonStudySession(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.StudySession{}, domain.ErrSessionNotFound
	}
	return s, err
}

func (r *studySessionRepository) IncrementCompletedCards(ctx context.Context, id uuid.UUID) (db.StudySession, error) {
	return r.q.IncrementCompletedCards(ctx, id)
}

func (r *studySessionRepository) GetMostRecentSession(ctx context.Context, userID uuid.UUID) (db.StudySession, error) {
	s, err := r.q.GetMostRecentSession(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.StudySession{}, domain.ErrSessionNotFound
	}
	return s, err
}

func (r *studySessionRepository) ListRecentDecks(ctx context.Context, userID uuid.UUID) ([]db.ListRecentDecksRow, error) {
	return r.q.ListRecentDecks(ctx, userID)
}
