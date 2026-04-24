package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
)

type VerificationTokenRepository interface {
	CreateVerificationToken(ctx context.Context, userID uuid.UUID, tokenHash, tokenType string, expiresAt time.Time) (db.VerificationToken, error)
	GetByHash(ctx context.Context, tokenHash string) (db.VerificationToken, error)
	MarkUsed(ctx context.Context, tokenHash string) error
	DeleteExpiredForUser(ctx context.Context, userID uuid.UUID) error
}

type verificationTokenRepository struct {
	q *db.Queries
}

func NewVerificationTokenRepository(database *sql.DB) VerificationTokenRepository {
	return &verificationTokenRepository{q: db.New(database)}
}

func (r *verificationTokenRepository) CreateVerificationToken(ctx context.Context, userID uuid.UUID, tokenHash, tokenType string, expiresAt time.Time) (db.VerificationToken, error) {
	return r.q.CreateVerificationToken(ctx, db.CreateVerificationTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		Type:      tokenType,
		ExpiresAt: expiresAt,
	})
}

func (r *verificationTokenRepository) GetByHash(ctx context.Context, tokenHash string) (db.VerificationToken, error) {
	vt, err := r.q.GetVerificationTokenByHash(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return db.VerificationToken{}, domain.ErrTokenNotFound
	}
	return vt, err
}

func (r *verificationTokenRepository) MarkUsed(ctx context.Context, tokenHash string) error {
	return r.q.MarkVerificationTokenUsed(ctx, tokenHash)
}

func (r *verificationTokenRepository) DeleteExpiredForUser(ctx context.Context, userID uuid.UUID) error {
	return r.q.DeleteExpiredVerificationTokens(ctx, userID)
}
