package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/notification-service/internal/db"
	"mem_pan/services/notification-service/internal/domain"
)

type NotificationRepository interface {
	UpsertFCMToken(ctx context.Context, userID uuid.UUID, token, deviceName string) (db.FcmToken, error)
	DeleteFCMToken(ctx context.Context, userID uuid.UUID, token string) error
	ListFCMTokensByUser(ctx context.Context, userID uuid.UUID) ([]db.FcmToken, error)
	LogNotification(ctx context.Context, arg db.LogNotificationParams) error
	// Used by reminder cron handlers to avoid double-sending. Returns the
	// number of rows in notification_log for (user, type) created at or after
	// `since`.
	CountRecentNotifications(ctx context.Context, userID uuid.UUID, notifType string, since time.Time) (int64, error)

	// Email templates
	GetActiveEmailTemplate(ctx context.Context, key, locale string) (db.EmailTemplate, error)
	GetEmailTemplate(ctx context.Context, key, locale string) (db.EmailTemplate, error)
	ListEmailTemplates(ctx context.Context) ([]db.EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, arg db.UpdateEmailTemplateParams) (db.EmailTemplate, error)
	InsertEmailTemplateVersion(ctx context.Context, arg db.InsertEmailTemplateVersionParams) error
}

type postgresRepo struct {
	q *db.Queries
}

func New(database *sql.DB) NotificationRepository {
	return &postgresRepo{q: db.New(database)}
}

func (r *postgresRepo) GetActiveEmailTemplate(ctx context.Context, key, locale string) (db.EmailTemplate, error) {
	t, err := r.q.GetActiveEmailTemplate(ctx, db.GetActiveEmailTemplateParams{TemplateKey: key, Locale: locale})
	if errors.Is(err, sql.ErrNoRows) {
		return db.EmailTemplate{}, domain.ErrTemplateNotFound
	}
	return t, err
}

func (r *postgresRepo) GetEmailTemplate(ctx context.Context, key, locale string) (db.EmailTemplate, error) {
	t, err := r.q.GetEmailTemplate(ctx, db.GetEmailTemplateParams{TemplateKey: key, Locale: locale})
	if errors.Is(err, sql.ErrNoRows) {
		return db.EmailTemplate{}, domain.ErrTemplateNotFound
	}
	return t, err
}

func (r *postgresRepo) ListEmailTemplates(ctx context.Context) ([]db.EmailTemplate, error) {
	return r.q.ListEmailTemplates(ctx)
}

func (r *postgresRepo) UpdateEmailTemplate(ctx context.Context, arg db.UpdateEmailTemplateParams) (db.EmailTemplate, error) {
	t, err := r.q.UpdateEmailTemplate(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.EmailTemplate{}, domain.ErrTemplateNotFound
	}
	return t, err
}

func (r *postgresRepo) InsertEmailTemplateVersion(ctx context.Context, arg db.InsertEmailTemplateVersionParams) error {
	return r.q.InsertEmailTemplateVersion(ctx, arg)
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

func (r *postgresRepo) CountRecentNotifications(ctx context.Context, userID uuid.UUID, notifType string, since time.Time) (int64, error) {
	uid := userID
	return r.q.CountRecentNotifications(ctx, db.CountRecentNotificationsParams{
		UserID:           &uid,
		NotificationType: notifType,
		CreatedAt:        since,
	})
}
