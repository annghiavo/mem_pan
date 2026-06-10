package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"mem_pan/services/billing-service/internal/db"
	"mem_pan/services/billing-service/internal/domain"
)

type BillingRepository interface {
	CreateSubscription(ctx context.Context, userID uuid.UUID, planCode string, start, end time.Time) (db.Subscription, error)
	GetSubscriptionByID(ctx context.Context, subscriptionID uuid.UUID) (db.Subscription, error)
	GetLatestSubscriptionForUser(ctx context.Context, userID uuid.UUID) (db.Subscription, error)
	GetActiveSubscriptionForUser(ctx context.Context, userID uuid.UUID) (db.Subscription, error)
	ActivateSubscription(ctx context.Context, subscriptionID uuid.UUID, start, end time.Time) (db.Subscription, error)
	UpdateSubscriptionStatus(ctx context.Context, subscriptionID uuid.UUID, status db.SubscriptionStatus) (db.Subscription, error)
	ExpireSubscriptions(ctx context.Context) error

	CreatePaymentTransaction(ctx context.Context, arg db.CreatePaymentTransactionParams) (db.PaymentTransaction, error)
	GetPaymentTransactionByOrderCode(ctx context.Context, orderCode int64) (db.PaymentTransaction, error)
	MarkPaymentTransactionPaid(ctx context.Context, transactionID uuid.UUID, paidAt *time.Time, rawPayload []byte) (db.PaymentTransaction, error)
	MarkPaymentTransactionStatus(ctx context.Context, transactionID uuid.UUID, status db.PaymentStatus, rawPayload []byte) (db.PaymentTransaction, error)
	RecordWebhookEvent(ctx context.Context, eventKey string, rawPayload []byte) error

	GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error)
	GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error)
	GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error)
	MarkCreatorEarningPaid(ctx context.Context, earningID uuid.UUID) (db.CreatorEarning, error)
}

type postgresRepo struct {
	q *db.Queries
}

func New(database *sql.DB) BillingRepository {
	return &postgresRepo{q: db.New(database)}
}

func (r *postgresRepo) CreateSubscription(ctx context.Context, userID uuid.UUID, planCode string, start, end time.Time) (db.Subscription, error) {
	return r.q.CreateSubscription(ctx, db.CreateSubscriptionParams{
		UserID:             userID,
		PlanCode:           planCode,
		CurrentPeriodStart: start,
		CurrentPeriodEnd:   end,
	})
}

func (r *postgresRepo) GetSubscriptionByID(ctx context.Context, subscriptionID uuid.UUID) (db.Subscription, error) {
	row, err := r.q.GetSubscriptionByID(ctx, subscriptionID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Subscription{}, domain.ErrSubscriptionNotFound
	}
	return row, err
}

func (r *postgresRepo) GetLatestSubscriptionForUser(ctx context.Context, userID uuid.UUID) (db.Subscription, error) {
	row, err := r.q.GetLatestSubscriptionForUser(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Subscription{}, domain.ErrSubscriptionNotFound
	}
	return row, err
}

func (r *postgresRepo) GetActiveSubscriptionForUser(ctx context.Context, userID uuid.UUID) (db.Subscription, error) {
	row, err := r.q.GetActiveSubscriptionForUser(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Subscription{}, domain.ErrSubscriptionNotFound
	}
	return row, err
}

func (r *postgresRepo) ActivateSubscription(ctx context.Context, subscriptionID uuid.UUID, start, end time.Time) (db.Subscription, error) {
	return r.q.ActivateSubscription(ctx, db.ActivateSubscriptionParams{
		SubscriptionID:     subscriptionID,
		CurrentPeriodStart: start,
		CurrentPeriodEnd:   end,
	})
}

func (r *postgresRepo) UpdateSubscriptionStatus(ctx context.Context, subscriptionID uuid.UUID, status db.SubscriptionStatus) (db.Subscription, error) {
	return r.q.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		SubscriptionID: subscriptionID,
		Status:         status,
	})
}

func (r *postgresRepo) ExpireSubscriptions(ctx context.Context) error {
	return r.q.ExpireSubscriptions(ctx)
}

func (r *postgresRepo) CreatePaymentTransaction(ctx context.Context, arg db.CreatePaymentTransactionParams) (db.PaymentTransaction, error) {
	return r.q.CreatePaymentTransaction(ctx, arg)
}

func (r *postgresRepo) GetPaymentTransactionByOrderCode(ctx context.Context, orderCode int64) (db.PaymentTransaction, error) {
	row, err := r.q.GetPaymentTransactionByOrderCode(ctx, orderCode)
	if errors.Is(err, sql.ErrNoRows) {
		return db.PaymentTransaction{}, domain.ErrPaymentNotFound
	}
	return row, err
}

func (r *postgresRepo) MarkPaymentTransactionPaid(ctx context.Context, transactionID uuid.UUID, paidAt *time.Time, rawPayload []byte) (db.PaymentTransaction, error) {
	var paid sql.NullTime
	if paidAt != nil {
		paid = sql.NullTime{Time: *paidAt, Valid: true}
	}
	return r.q.MarkPaymentTransactionPaid(ctx, db.MarkPaymentTransactionPaidParams{
		TransactionID: transactionID,
		PaidAt:        paid,
		RawPayload:    nullableRaw(rawPayload),
	})
}

func (r *postgresRepo) MarkPaymentTransactionStatus(ctx context.Context, transactionID uuid.UUID, status db.PaymentStatus, rawPayload []byte) (db.PaymentTransaction, error) {
	return r.q.MarkPaymentTransactionStatus(ctx, db.MarkPaymentTransactionStatusParams{
		TransactionID: transactionID,
		Status:        status,
		RawPayload:    nullableRaw(rawPayload),
	})
}

func (r *postgresRepo) RecordWebhookEvent(ctx context.Context, eventKey string, rawPayload []byte) error {
	_, err := r.q.RecordWebhookEvent(ctx, db.RecordWebhookEventParams{
		EventKey:   eventKey,
		RawPayload: json.RawMessage(rawPayload),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrDuplicateWebhook
	}
	return err
}

func nullableRaw(raw []byte) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{
		RawMessage: json.RawMessage(raw),
		Valid:      len(raw) > 0,
	}
}

func (r *postgresRepo) GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error) {
	return r.q.GetMonthlyRevenuePools(ctx)
}

func (r *postgresRepo) GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error) {
	return r.q.GetCreatorEarningsByMonth(ctx, poolMonth)
}

func (r *postgresRepo) GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error) {
	return r.q.GetMyEarnings(ctx, creatorID)
}

func (r *postgresRepo) MarkCreatorEarningPaid(ctx context.Context, earningID uuid.UUID) (db.CreatorEarning, error) {
	return r.q.MarkCreatorEarningPaid(ctx, earningID)
}
