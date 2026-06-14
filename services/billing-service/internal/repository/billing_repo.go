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
	SyncRevenuePool(ctx context.Context, pool db.UpsertMonthlyRevenuePoolParams, earnings []db.UpsertCreatorEarningParams) error
	GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error)
	GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error)
	GetCreatorBalanceSummary(ctx context.Context, creatorID uuid.UUID) (db.GetCreatorBalanceSummaryRow, error)
	ListCreatorBalanceHistory(ctx context.Context, arg db.ListCreatorBalanceHistoryParams) ([]db.ListCreatorBalanceHistoryRow, error)
	GetCreatorEarningByID(ctx context.Context, earningID uuid.UUID) (db.CreatorEarning, error)
	CreateCreatorWithdrawal(ctx context.Context, arg db.CreateCreatorWithdrawalParams) (db.CreatorWithdrawal, error)
	GetCreatorWithdrawalByID(ctx context.Context, withdrawalID uuid.UUID) (db.CreatorWithdrawal, error)
	UpdateCreatorWithdrawalStatus(ctx context.Context, arg db.UpdateCreatorWithdrawalStatusParams) (db.CreatorWithdrawal, error)
	UpsertCreatorWithdrawalReservation(ctx context.Context, arg db.UpsertCreatorWithdrawalReservationParams) (db.CreatorBalanceTransaction, error)
	UpdateCreatorBalanceTransactionStatus(ctx context.Context, arg db.UpdateCreatorBalanceTransactionStatusParams) (db.CreatorBalanceTransaction, error)
	MarkCreatorEarningPayoutProcessing(ctx context.Context, arg db.MarkCreatorEarningPayoutProcessingParams) (db.CreatorEarning, error)
	MarkCreatorEarningPayoutPaid(ctx context.Context, arg db.MarkCreatorEarningPayoutPaidParams) (db.CreatorEarning, error)
	MarkCreatorEarningPayoutFailed(ctx context.Context, arg db.MarkCreatorEarningPayoutFailedParams) (db.CreatorEarning, error)
	UpsertCreatorPayoutAccount(ctx context.Context, arg db.UpsertCreatorPayoutAccountParams) (db.CreatorPayoutAccount, error)
	GetCreatorPayoutAccount(ctx context.Context, creatorID uuid.UUID) (db.CreatorPayoutAccount, error)
}

type postgresRepo struct {
	db *sql.DB
	q  *db.Queries
}

func New(database *sql.DB) BillingRepository {
	return &postgresRepo{db: database, q: db.New(database)}
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
		RawPayload:    validJSONRaw(rawPayload),
	})
}

func (r *postgresRepo) MarkPaymentTransactionStatus(ctx context.Context, transactionID uuid.UUID, status db.PaymentStatus, rawPayload []byte) (db.PaymentTransaction, error) {
	return r.q.MarkPaymentTransactionStatus(ctx, db.MarkPaymentTransactionStatusParams{
		TransactionID: transactionID,
		Status:        status,
		RawPayload:    validJSONRaw(rawPayload),
	})
}

func (r *postgresRepo) RecordWebhookEvent(ctx context.Context, eventKey string, rawPayload []byte) error {
	_, err := r.q.RecordWebhookEvent(ctx, db.RecordWebhookEventParams{
		EventKey:   eventKey,
		RawPayload: validJSONRaw(rawPayload),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrDuplicateWebhook
	}
	return err
}

func nullableRaw(raw []byte) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{
		RawMessage: raw,
		Valid:      len(raw) > 0 && json.Valid(raw),
	}
}

func validJSONRaw(raw []byte) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage("null")
	}
	return json.RawMessage(raw)
}

func (r *postgresRepo) GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error) {
	return r.q.GetMonthlyRevenuePools(ctx)
}

func (r *postgresRepo) SyncRevenuePool(ctx context.Context, pool db.UpsertMonthlyRevenuePoolParams, earnings []db.UpsertCreatorEarningParams) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	qtx := r.q.WithTx(tx)
	if _, err := qtx.UpsertMonthlyRevenuePool(ctx, pool); err != nil {
		return err
	}
	for _, earning := range earnings {
		row, err := qtx.UpsertCreatorEarning(ctx, earning)
		if err != nil {
			return err
		}
		if _, err := qtx.UpsertCreatorEarningCreditTransaction(ctx, db.UpsertCreatorEarningCreditTransactionParams{
			CreatorID: row.CreatorID,
			SourceID:  row.EarningID.String(),
			AmountVnd: row.AmountVnd,
			PoolMonth: sql.NullTime{Time: row.PoolMonth, Valid: true},
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *postgresRepo) GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error) {
	return r.q.GetCreatorEarningsByMonth(ctx, poolMonth)
}

func (r *postgresRepo) GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error) {
	return r.q.GetMyEarnings(ctx, creatorID)
}

func (r *postgresRepo) GetCreatorBalanceSummary(ctx context.Context, creatorID uuid.UUID) (db.GetCreatorBalanceSummaryRow, error) {
	return r.q.GetCreatorBalanceSummary(ctx, creatorID)
}

func (r *postgresRepo) ListCreatorBalanceHistory(ctx context.Context, arg db.ListCreatorBalanceHistoryParams) ([]db.ListCreatorBalanceHistoryRow, error) {
	return r.q.ListCreatorBalanceHistory(ctx, arg)
}

func (r *postgresRepo) GetCreatorEarningByID(ctx context.Context, earningID uuid.UUID) (db.CreatorEarning, error) {
	row, err := r.q.GetCreatorEarningByID(ctx, earningID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorEarning{}, domain.ErrEarningNotFound
	}
	return row, err
}

func (r *postgresRepo) CreateCreatorWithdrawal(ctx context.Context, arg db.CreateCreatorWithdrawalParams) (db.CreatorWithdrawal, error) {
	return r.q.CreateCreatorWithdrawal(ctx, arg)
}

func (r *postgresRepo) GetCreatorWithdrawalByID(ctx context.Context, withdrawalID uuid.UUID) (db.CreatorWithdrawal, error) {
	row, err := r.q.GetCreatorWithdrawalByID(ctx, withdrawalID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorWithdrawal{}, domain.ErrWithdrawalNotFound
	}
	return row, err
}

func (r *postgresRepo) UpdateCreatorWithdrawalStatus(ctx context.Context, arg db.UpdateCreatorWithdrawalStatusParams) (db.CreatorWithdrawal, error) {
	row, err := r.q.UpdateCreatorWithdrawalStatus(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorWithdrawal{}, domain.ErrWithdrawalNotFound
	}
	return row, err
}

func (r *postgresRepo) UpsertCreatorWithdrawalReservation(ctx context.Context, arg db.UpsertCreatorWithdrawalReservationParams) (db.CreatorBalanceTransaction, error) {
	return r.q.UpsertCreatorWithdrawalReservation(ctx, arg)
}

func (r *postgresRepo) UpdateCreatorBalanceTransactionStatus(ctx context.Context, arg db.UpdateCreatorBalanceTransactionStatusParams) (db.CreatorBalanceTransaction, error) {
	row, err := r.q.UpdateCreatorBalanceTransactionStatus(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorBalanceTransaction{}, domain.ErrWithdrawalNotFound
	}
	return row, err
}

func (r *postgresRepo) MarkCreatorEarningPayoutProcessing(ctx context.Context, arg db.MarkCreatorEarningPayoutProcessingParams) (db.CreatorEarning, error) {
	row, err := r.q.MarkCreatorEarningPayoutProcessing(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorEarning{}, domain.ErrPayoutNotAllowed
	}
	return row, err
}

func (r *postgresRepo) MarkCreatorEarningPayoutPaid(ctx context.Context, arg db.MarkCreatorEarningPayoutPaidParams) (db.CreatorEarning, error) {
	row, err := r.q.MarkCreatorEarningPayoutPaid(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorEarning{}, domain.ErrEarningNotFound
	}
	return row, err
}

func (r *postgresRepo) MarkCreatorEarningPayoutFailed(ctx context.Context, arg db.MarkCreatorEarningPayoutFailedParams) (db.CreatorEarning, error) {
	row, err := r.q.MarkCreatorEarningPayoutFailed(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorEarning{}, domain.ErrEarningNotFound
	}
	return row, err
}

func (r *postgresRepo) UpsertCreatorPayoutAccount(ctx context.Context, arg db.UpsertCreatorPayoutAccountParams) (db.CreatorPayoutAccount, error) {
	return r.q.UpsertCreatorPayoutAccount(ctx, arg)
}

func (r *postgresRepo) GetCreatorPayoutAccount(ctx context.Context, creatorID uuid.UUID) (db.CreatorPayoutAccount, error) {
	row, err := r.q.GetCreatorPayoutAccount(ctx, creatorID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.CreatorPayoutAccount{}, domain.ErrPayoutAccountNotFound
	}
	return row, err
}
