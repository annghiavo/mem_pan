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

type RefreshTokenRepository interface {
	CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, userAgent, ipAddress *string, expiresAt time.Time) (db.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error)
	RevokeByHash(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredForUser(ctx context.Context, userID uuid.UUID) error
}

type refreshTokenRepository struct {
	q *db.Queries
}

func NewRefreshTokenRepository(database *sql.DB) RefreshTokenRepository {
	return &refreshTokenRepository{q: db.New(database)}
}

func (r *refreshTokenRepository) CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, userAgent, ipAddress *string, expiresAt time.Time) (db.RefreshToken, error) {
	arg := db.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	if userAgent != nil {
		arg.UserAgent = sql.NullString{String: *userAgent, Valid: true}
	}
	if ipAddress != nil {
		arg.IpAddress = sql.NullString{String: *ipAddress, Valid: true}
	}
	return r.q.CreateRefreshToken(ctx, arg)
}

func (r *refreshTokenRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	rt, err := r.q.GetRefreshTokenByHash(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return db.RefreshToken{}, domain.ErrTokenNotFound
	}
	return rt, err
}

func (r *refreshTokenRepository) RevokeByHash(ctx context.Context, tokenHash string) error {
	return r.q.RevokeRefreshToken(ctx, tokenHash)
}

func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	return r.q.RevokeAllUserRefreshTokens(ctx, userID)
}

func (r *refreshTokenRepository) DeleteExpiredForUser(ctx context.Context, userID uuid.UUID) error {
	return r.q.DeleteExpiredRefreshTokens(ctx, userID)
}
