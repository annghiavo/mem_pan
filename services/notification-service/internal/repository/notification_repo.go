package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"mem_pan/services/notification-service/internal/db"
)

type NotificationRepository interface {
	UpsertFCMToken(ctx context.Context, userID uuid.UUID, token, deviceName string) (db.FcmToken, error)
	DeleteFCMToken(ctx context.Context, userID uuid.UUID, token string) error
	ListFCMTokensByUser(ctx context.Context, userID uuid.UUID) ([]db.FcmToken, error)
	LogNotification(ctx context.Context, arg db.LogNotificationParams) error
}

type postgresRepo struct {
	q *db.Queries
}

func New(database *sql.DB) NotificationRepository {
	return &postgresRepo{q: db.New(database)}
}

func (r *postgresRepo) UpsertFCMToken(ctx context.Context, userID uuid.UUID, token, deviceName string) (db.FcmToken, error) {
	return r.q.UpsertFCMToken(ctx, db.UpsertFCMTokenParams{
		UserID:     userID,
		Token:      token,
		DeviceName: deviceName,
	})
}

func (r *postgresRepo) DeleteFCMToken(ctx context.Context, userID uuid.UUID, token string) error {
	return r.q.DeleteFCMToken(ctx, db.DeleteFCMTokenParams{Token: token, UserID: userID})
}

func (r *postgresRepo) ListFCMTokensByUser(ctx context.Context, userID uuid.UUID) ([]db.FcmToken, error) {
	return r.q.ListFCMTokensByUser(ctx, userID)
}

func (r *postgresRepo) LogNotification(ctx context.Context, arg db.LogNotificationParams) error {
	return r.q.LogNotification(ctx, arg)
}
