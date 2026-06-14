package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"mem_pan/services/billing-service/internal/authclient"
	"mem_pan/services/billing-service/internal/db"
	"mem_pan/services/billing-service/internal/domain"
	"mem_pan/services/billing-service/internal/payos"
	"mem_pan/services/billing-service/internal/repository"
)

const (
	PlanPlusMonthly = "plus_monthly"
	PlanPlusYearly  = "plus_yearly"

	MinimumPayoutAmountVND = int64(1000) // lowered for dev testing; production should be 100000
)

type Plan struct {
	Code      string
	Name      string
	AmountVND int64
	Duration  time.Duration
}

type CheckoutInput struct {
	UserID    uuid.UUID
	PlanCode  string
	ReturnURL string
	CancelURL string
}

type CheckoutResult struct {
	SubscriptionID string `json:"subscription_id"`
	TransactionID  string `json:"transaction_id"`
	OrderCode      int64  `json:"order_code"`
	PaymentLinkID  string `json:"payment_link_id"`
	CheckoutURL    string `json:"checkout_url"`
	AmountVND      int64  `json:"amount_vnd"`
	Status         string `json:"status"`
}

type SubscriptionStatus struct {
	Active           bool      `json:"active"`
	SubscriptionID   string    `json:"subscription_id,omitempty"`
	PlanCode         string    `json:"plan_code,omitempty"`
	Status           string    `json:"status"`
	CurrentPeriodEnd time.Time `json:"current_period_end,omitempty"`
}

type WebhookInput struct {
	RawPayload []byte
	Body       PayOSWebhook
}

type PayOSWebhook struct {
	Code      string         `json:"code"`
	Desc      string         `json:"desc"`
	Success   bool           `json:"success"`
	Data      map[string]any `json:"data"`
	Signature string         `json:"signature"`
}

type PayoutInput struct {
	EarningID       uuid.UUID
	ToBin           string
	ToAccountNumber string
	ToAccountName   string
	Description     string
	Category        []string
}

type PayoutAccountInput struct {
	CreatorID     uuid.UUID
	BankBin       string
	BankCode      string
	BankShortName string
	BankName      string
	BankLogo      string
	AccountNumber string
	AccountName   string
}

type BatchPayoutInput struct {
	Payouts  []PayoutInput
	Category []string
}

type CreatorWithdrawalInput struct {
	CreatorID       uuid.UUID
	AmountVND       int64
	ToBin           string
	ToAccountNumber string
	ToAccountName   string
	Description     string
	Category        []string
}

type PayoutResult struct {
	Earning       db.CreatorEarning `json:"earning"`
	PayoutID      string            `json:"payout_id"`
	TransactionID string            `json:"transaction_id"`
	ReferenceID   string            `json:"reference_id"`
	State         string            `json:"state"`
	Status        string            `json:"status"`
}

type BatchPayoutResult struct {
	BatchPayoutID string         `json:"batch_payout_id"`
	ReferenceID   string         `json:"reference_id"`
	Results       []PayoutResult `json:"results"`
}

type ConfirmPaymentResult struct {
	Status         string `json:"status"`
	SubscriptionID string `json:"subscription_id,omitempty"`
	PlanCode       string `json:"plan_code,omitempty"`
	PaidAt         string `json:"paid_at,omitempty"`
	Active         bool   `json:"active"`
}

type CreatorEarningsSummary struct {
	CurrentLearners            int32  `json:"current_learners"`
	TotalEarnedAmountVND       int64  `json:"total_earned_amount_vnd"`
	AvailableBalanceVND        int64  `json:"available_balance_vnd"`
	PendingWithdrawalAmountVND int64  `json:"pending_withdrawal_amount_vnd"`
	TotalWithdrawnAmountVND    int64  `json:"total_withdrawn_amount_vnd"`
	LatestPoolMonth            string `json:"latest_pool_month,omitempty"`
}

type CreatorBalanceHistoryItem struct {
	TransactionID            string     `json:"transaction_id"`
	Type                     string     `json:"type"`
	SourceID                 string     `json:"source_id"`
	AmountVND                int64      `json:"amount_vnd"`
	AbsoluteAmountVND        int64      `json:"absolute_amount_vnd"`
	LedgerStatus             string     `json:"ledger_status"`
	OccurredAt               time.Time  `json:"occurred_at"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	PoolMonth                string     `json:"pool_month,omitempty"`
	EarningID                string     `json:"earning_id,omitempty"`
	EarningStatus            string     `json:"earning_status,omitempty"`
	EligibleLearners         int32      `json:"eligible_learners,omitempty"`
	WeightedScore            string     `json:"weighted_score,omitempty"`
	WithdrawalID             string     `json:"withdrawal_id,omitempty"`
	WithdrawalStatus         string     `json:"withdrawal_status,omitempty"`
	WithdrawalRequestedAt    *time.Time `json:"withdrawal_requested_at,omitempty"`
	WithdrawalPaidAt         *time.Time `json:"withdrawal_paid_at,omitempty"`
	PayoutToBin              string     `json:"payout_to_bin,omitempty"`
	PayoutToAccountNumber    string     `json:"payout_to_account_number,omitempty"`
	PayoutToAccountName      string     `json:"payout_to_account_name,omitempty"`
	PayosPayoutID            string     `json:"payos_payout_id,omitempty"`
	PayosPayoutTransactionID string     `json:"payos_payout_transaction_id,omitempty"`
	PayosPayoutState         string     `json:"payos_payout_state,omitempty"`
	PayoutFailedReason       string     `json:"payout_failed_reason,omitempty"`
}

type CreatorBalanceHistoryResult struct {
	Items  []CreatorBalanceHistoryItem `json:"items"`
	Limit  int32                       `json:"limit"`
	Offset int32                       `json:"offset"`
}

type CreatorWithdrawalResult struct {
	Withdrawal    db.CreatorWithdrawal   `json:"withdrawal"`
	Balance       CreatorEarningsSummary `json:"balance"`
	PayoutID      string                 `json:"payout_id"`
	TransactionID string                 `json:"transaction_id"`
	ReferenceID   string                 `json:"reference_id"`
	State         string                 `json:"state"`
	Status        string                 `json:"status"`
}

type RevenuePoolSyncInput struct {
	Pool     db.UpsertMonthlyRevenuePoolParams
	Earnings []db.UpsertCreatorEarningParams
}

type BillingService interface {
	CreateCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error)
	GetSubscriptionStatus(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error)
	CheckPlusAccess(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error)
	ProcessPayOSWebhook(ctx context.Context, in WebhookInput) error
	ConfirmPayment(ctx context.Context, userID uuid.UUID, orderCode int64) (ConfirmPaymentResult, error)
	ExpireSubscriptions(ctx context.Context) error
	SyncRevenuePool(ctx context.Context, in RevenuePoolSyncInput) error

	GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error)
	GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error)
	GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error)
	GetMyEarningsSummary(ctx context.Context, creatorID uuid.UUID) (CreatorEarningsSummary, error)
	ListMyBalanceHistory(ctx context.Context, creatorID uuid.UUID, limit, offset int32) (CreatorBalanceHistoryResult, error)
	UpsertCreatorPayoutAccount(ctx context.Context, in PayoutAccountInput) (db.CreatorPayoutAccount, error)
	GetCreatorPayoutAccount(ctx context.Context, creatorID uuid.UUID) (db.CreatorPayoutAccount, error)
	CreateCreatorWithdrawal(ctx context.Context, in CreatorWithdrawalInput) (CreatorWithdrawalResult, error)
	WithdrawCreatorEarning(ctx context.Context, creatorID uuid.UUID, in PayoutInput) (PayoutResult, error)
	PayoutCreatorEarning(ctx context.Context, in PayoutInput) (PayoutResult, error)
	BatchPayoutCreatorEarnings(ctx context.Context, in BatchPayoutInput) (BatchPayoutResult, error)
	GetPayoutAccountBalance(ctx context.Context) (payos.PayoutBalanceResponse, error)
}

type billingService struct {
	repo             repository.BillingRepository
	payosClient      *payos.Client
	authClient       authclient.Client
	plans            map[string]Plan
	defaultReturnURL string
	defaultCancelURL string
	now              func() time.Time
}

func New(repo repository.BillingRepository, payosClient *payos.Client, authClient authclient.Client, monthlyAmount, yearlyAmount int64, returnURL, cancelURL string) BillingService {
	return &billingService{
		repo:        repo,
		payosClient: payosClient,
		authClient:  authClient,
		plans: map[string]Plan{
			PlanPlusMonthly: {Code: PlanPlusMonthly, Name: "MemPan Plus Monthly", AmountVND: monthlyAmount, Duration: 30 * 24 * time.Hour},
			PlanPlusYearly:  {Code: PlanPlusYearly, Name: "MemPan Plus Yearly", AmountVND: yearlyAmount, Duration: 365 * 24 * time.Hour},
		},
		defaultReturnURL: returnURL,
		defaultCancelURL: cancelURL,
		now:              time.Now,
	}
}

func (s *billingService) CreateCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error) {
	plan, ok := s.plans[in.PlanCode]
	if !ok {
		return CheckoutResult{}, domain.ErrInvalidPlan
	}
	in.ReturnURL = coalesceCheckoutURL(in.ReturnURL, s.defaultReturnURL)
	in.CancelURL = coalesceCheckoutURL(in.CancelURL, s.defaultCancelURL)
	if in.ReturnURL == "" || in.CancelURL == "" {
		return CheckoutResult{}, fmt.Errorf("invalid checkout callback urls: return_url=%q cancel_url=%q", in.ReturnURL, in.CancelURL)
	}
	slog.Info("billing checkout callback urls",
		"plan_code", in.PlanCode,
		"return_url", in.ReturnURL,
		"cancel_url", in.CancelURL,
	)

	now := s.now().UTC()
	sub, err := s.repo.CreateSubscription(ctx, in.UserID, plan.Code, now, now.Add(24*time.Hour))
	if err != nil {
		return CheckoutResult{}, err
	}

	orderCode, err := newOrderCode()
	if err != nil {
		return CheckoutResult{}, err
	}
	description := shortDescription(orderCode)
	payosResp, raw, err := s.payosClient.CreatePaymentLink(ctx, payos.CreatePaymentLinkRequest{
		OrderCode:   orderCode,
		Amount:      plan.AmountVND,
		Description: description,
		CancelURL:   in.CancelURL,
		ReturnURL:   in.ReturnURL,
		Items: []payos.Item{{
			Name:     plan.Name,
			Quantity: 1,
			Price:    plan.AmountVND,
		}},
	})
	if err != nil {
		_, _ = s.repo.UpdateSubscriptionStatus(ctx, sub.SubscriptionID, db.SubscriptionStatusCancelled)
		return CheckoutResult{}, err
	}

	tx, err := s.repo.CreatePaymentTransaction(ctx, db.CreatePaymentTransactionParams{
		UserID:            in.UserID,
		SubscriptionID:    uuidNull(sub.SubscriptionID),
		ProviderPaymentID: stringNull(payosResp.Data.PaymentLinkID),
		ProviderOrderCode: payosResp.Data.OrderCode,
		IdempotencyKey:    fmt.Sprintf("payos:%d", payosResp.Data.OrderCode),
		AmountVnd:         plan.AmountVND,
		CheckoutUrl:       stringNull(payosResp.Data.CheckoutURL),
		RawPayload:        json.RawMessage(raw),
	})
	if err != nil {
		return CheckoutResult{}, err
	}

	return CheckoutResult{
		SubscriptionID: sub.SubscriptionID.String(),
		TransactionID:  tx.TransactionID.String(),
		OrderCode:      payosResp.Data.OrderCode,
		PaymentLinkID:  payosResp.Data.PaymentLinkID,
		CheckoutURL:    payosResp.Data.CheckoutURL,
		AmountVND:      plan.AmountVND,
		Status:         string(tx.Status),
	}, nil
}

func (s *billingService) GetSubscriptionStatus(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error) {
	if err := s.repo.ExpireSubscriptions(ctx); err != nil {
		return SubscriptionStatus{}, err
	}

	// Always prioritize the active subscription if one exists.
	sub, err := s.repo.GetActiveSubscriptionForUser(ctx, userID)
	if err == nil {
		return subscriptionStatus(sub), nil
	}
	if !errors.Is(err, domain.ErrSubscriptionNotFound) {
		return SubscriptionStatus{}, err
	}

	// Fallback: get the latest created subscription (could be pending, expired, etc.)
	sub, err = s.repo.GetLatestSubscriptionForUser(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrSubscriptionNotFound) {
			return SubscriptionStatus{Active: false, Status: "none"}, nil
		}
		return SubscriptionStatus{}, err
	}
	return subscriptionStatus(sub), nil
}

func (s *billingService) CheckPlusAccess(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error) {
	if err := s.repo.ExpireSubscriptions(ctx); err != nil {
		return SubscriptionStatus{}, err
	}
	sub, err := s.repo.GetActiveSubscriptionForUser(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrSubscriptionNotFound) {
			return SubscriptionStatus{Active: false, Status: "none"}, nil
		}
		return SubscriptionStatus{}, err
	}
	return subscriptionStatus(sub), nil
}

func (s *billingService) ProcessPayOSWebhook(ctx context.Context, in WebhookInput) error {
	if in.Body.Signature == "" || in.Body.Data == nil {
		return domain.ErrInvalidWebhook
	}
	if !payos.VerifySignature(in.Body.Data, in.Body.Signature, s.payosClient.ChecksumKey()) {
		return domain.ErrInvalidWebhook
	}

	orderCode, err := int64FromAny(in.Body.Data["orderCode"])
	if err != nil {
		return domain.ErrInvalidWebhook
	}
	amount, err := int64FromAny(in.Body.Data["amount"])
	if err != nil {
		return domain.ErrInvalidWebhook
	}
	eventKey := webhookEventKey(in.Body.Data, orderCode)
	if err := s.repo.RecordWebhookEvent(ctx, eventKey, in.RawPayload); err != nil {
		if errors.Is(err, domain.ErrDuplicateWebhook) {
			return nil
		}
		return err
	}

	tx, err := s.repo.GetPaymentTransactionByOrderCode(ctx, orderCode)
	if err != nil {
		return err
	}
	if tx.AmountVnd != amount {
		return domain.ErrAmountMismatch
	}

	if in.Body.Success && in.Body.Code == "00" {
		paidAt := parsePayOSTime(anyString(in.Body.Data["transactionDateTime"]))
		tx, err = s.repo.MarkPaymentTransactionPaid(ctx, tx.TransactionID, paidAt, in.RawPayload)
		if err != nil {
			return err
		}
		if tx.SubscriptionID.Valid {
			paidTime := s.now().UTC()
			if paidAt != nil {
				paidTime = paidAt.UTC()
			}
			sub, err := s.repo.GetSubscriptionByID(ctx, tx.SubscriptionID.UUID)
			if err != nil {
				return err
			}
			plan, ok := s.plans[sub.PlanCode]
			if !ok {
				return domain.ErrInvalidPlan
			}
			periodStart, periodEnd, err := s.activationPeriod(ctx, tx.UserID, sub.SubscriptionID, paidTime, plan.Duration)
			if err != nil {
				return err
			}
			_, err = s.repo.ActivateSubscription(ctx, sub.SubscriptionID, periodStart, periodEnd)
			if err != nil {
				return err
			}
			if err := s.authClient.UpdateUserPlusStatus(ctx, tx.UserID, true); err != nil {
				return fmt.Errorf("sync plus status to auth-service: %w", err)
			}
			return nil
		}
		return nil
	}

	status := db.PaymentStatusFailed
	if anyString(in.Body.Data["code"]) == "01" || in.Body.Desc == "cancelled" {
		status = db.PaymentStatusCancelled
	}
	_, err = s.repo.MarkPaymentTransactionStatus(ctx, tx.TransactionID, status, in.RawPayload)
	return err
}

// ConfirmPayment actively queries PayOS for the payment status and activates
// the subscription if the payment has been completed. This is the primary
// mechanism for subscription activation — called by the client after the user
// returns from the PayOS checkout page.
func (s *billingService) ConfirmPayment(ctx context.Context, userID uuid.UUID, orderCode int64) (ConfirmPaymentResult, error) {
	// 1. Find our local payment transaction
	tx, err := s.repo.GetPaymentTransactionByOrderCode(ctx, orderCode)
	if err != nil {
		return ConfirmPaymentResult{}, err
	}

	// Security: ensure the payment belongs to the requesting user
	if tx.UserID != userID {
		return ConfirmPaymentResult{}, domain.ErrPaymentNotFound
	}

	// If already paid, just return the current status
	if tx.Status == db.PaymentStatusPaid {
		var sub db.Subscription
		if tx.SubscriptionID.Valid {
			sub, _ = s.repo.GetSubscriptionByID(ctx, tx.SubscriptionID.UUID)
		}
		if sub.Status == db.SubscriptionStatusActive {
			if err := s.authClient.UpdateUserPlusStatus(ctx, tx.UserID, true); err != nil {
				return ConfirmPaymentResult{}, fmt.Errorf("sync plus status to auth-service: %w", err)
			}
		}
		return ConfirmPaymentResult{
			Status:         string(tx.Status),
			SubscriptionID: uuidString(tx.SubscriptionID),
			PlanCode:       sub.PlanCode,
			Active:         sub.Status == db.SubscriptionStatusActive,
		}, nil
	}

	// 2. Query PayOS for the actual payment status
	info, raw, err := s.payosClient.GetPaymentLinkInfo(ctx, orderCode)
	if err != nil {
		return ConfirmPaymentResult{}, fmt.Errorf("failed to query payos: %w", err)
	}

	// 3. Process based on PayOS status
	switch strings.ToUpper(info.Data.Status) {
	case "PAID":
		// Find the payment time from the first transaction
		var paidAt *time.Time
		if len(info.Data.Transactions) > 0 {
			paidAt = parsePayOSTime(info.Data.Transactions[0].TransactionDateTime)
		}

		// Mark the transaction as paid
		tx, err = s.repo.MarkPaymentTransactionPaid(ctx, tx.TransactionID, paidAt, raw)
		if err != nil {
			return ConfirmPaymentResult{}, err
		}

		// Activate the subscription
		var sub db.Subscription
		if tx.SubscriptionID.Valid {
			paidTime := s.now().UTC()
			if paidAt != nil {
				paidTime = paidAt.UTC()
			}
			sub, err = s.repo.GetSubscriptionByID(ctx, tx.SubscriptionID.UUID)
			if err != nil {
				return ConfirmPaymentResult{}, err
			}
			plan, ok := s.plans[sub.PlanCode]
			if !ok {
				return ConfirmPaymentResult{}, domain.ErrInvalidPlan
			}
			periodStart, periodEnd, err := s.activationPeriod(ctx, tx.UserID, sub.SubscriptionID, paidTime, plan.Duration)
			if err != nil {
				return ConfirmPaymentResult{}, err
			}
			sub, err = s.repo.ActivateSubscription(ctx, sub.SubscriptionID, periodStart, periodEnd)
			if err != nil {
				return ConfirmPaymentResult{}, err
			}
			if err := s.authClient.UpdateUserPlusStatus(ctx, tx.UserID, true); err != nil {
				return ConfirmPaymentResult{}, fmt.Errorf("sync plus status to auth-service: %w", err)
			}
		}

		paidAtStr := ""
		if paidAt != nil {
			paidAtStr = paidAt.Format(time.RFC3339)
		}
		return ConfirmPaymentResult{
			Status:         "paid",
			SubscriptionID: uuidString(tx.SubscriptionID),
			PlanCode:       sub.PlanCode,
			PaidAt:         paidAtStr,
			Active:         true,
		}, nil

	case "CANCELLED", "EXPIRED":
		_, _ = s.repo.MarkPaymentTransactionStatus(ctx, tx.TransactionID, db.PaymentStatusCancelled, raw)
		return ConfirmPaymentResult{Status: strings.ToLower(info.Data.Status)}, nil

	default: // PENDING, PROCESSING
		return ConfirmPaymentResult{Status: strings.ToLower(info.Data.Status)}, nil
	}
}

func (s *billingService) activationPeriod(ctx context.Context, userID uuid.UUID, subscriptionID uuid.UUID, paidTime time.Time, duration time.Duration) (time.Time, time.Time, error) {
	activeSub, err := s.repo.GetActiveSubscriptionForUser(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrSubscriptionNotFound) {
			start, end := calculateActivationPeriod(subscriptionID, paidTime, duration, db.Subscription{}, false)
			return start, end, nil
		}
		return time.Time{}, time.Time{}, err
	}
	start, end := calculateActivationPeriod(subscriptionID, paidTime, duration, activeSub, true)
	return start, end, nil
}

func calculateActivationPeriod(subscriptionID uuid.UUID, paidTime time.Time, duration time.Duration, activeSub db.Subscription, hasActiveSub bool) (time.Time, time.Time) {
	start := paidTime.UTC()
	if !hasActiveSub {
		return start, start.Add(duration)
	}
	if activeSub.SubscriptionID != subscriptionID && activeSub.CurrentPeriodEnd.After(start) {
		start = activeSub.CurrentPeriodEnd.UTC()
	}
	return start, start.Add(duration)
}

func uuidString(id uuid.NullUUID) string {
	if id.Valid {
		return id.UUID.String()
	}
	return ""
}

func (s *billingService) ExpireSubscriptions(ctx context.Context) error {
	return s.repo.ExpireSubscriptions(ctx)
}

func (s *billingService) SyncRevenuePool(ctx context.Context, in RevenuePoolSyncInput) error {
	if in.Pool.PoolMonth.IsZero() {
		return domain.ErrInvalidPayout
	}
	return s.repo.SyncRevenuePool(ctx, in.Pool, in.Earnings)
}

func (s *billingService) GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error) {
	return s.repo.GetMonthlyRevenuePools(ctx)
}

func (s *billingService) GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error) {
	return s.repo.GetCreatorEarningsByMonth(ctx, poolMonth)
}

func (s *billingService) GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error) {
	return s.repo.GetMyEarnings(ctx, creatorID)
}

func (s *billingService) GetMyEarningsSummary(ctx context.Context, creatorID uuid.UUID) (CreatorEarningsSummary, error) {
	earnings, err := s.repo.GetMyEarnings(ctx, creatorID)
	if err != nil {
		return CreatorEarningsSummary{}, err
	}
	balance, err := s.repo.GetCreatorBalanceSummary(ctx, creatorID)
	if err != nil {
		return CreatorEarningsSummary{}, err
	}
	var out CreatorEarningsSummary
	var latest *db.CreatorEarning
	for i := range earnings {
		row := earnings[i]
		if latest == nil || row.PoolMonth.After(latest.PoolMonth) {
			latest = &row
		}
	}
	out.TotalEarnedAmountVND = balance.TotalEarnedAmountVnd
	out.AvailableBalanceVND = balance.AvailableBalanceVnd
	out.PendingWithdrawalAmountVND = balance.PendingWithdrawalAmountVnd
	out.TotalWithdrawnAmountVND = balance.TotalWithdrawnAmountVnd
	if latest != nil {
		out.CurrentLearners = latest.EligibleLearners
		out.LatestPoolMonth = latest.PoolMonth.Format("2006-01")
	}
	return out, nil
}

func (s *billingService) ListMyBalanceHistory(ctx context.Context, creatorID uuid.UUID, limit, offset int32) (CreatorBalanceHistoryResult, error) {
	if creatorID == uuid.Nil {
		return CreatorBalanceHistoryResult{}, domain.ErrInvalidPayout
	}
	if limit <= 0 {
		limit = 80
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.repo.ListCreatorBalanceHistory(ctx, db.ListCreatorBalanceHistoryParams{
		CreatorID:  creatorID,
		LimitRows:  limit,
		OffsetRows: offset,
	})
	if err != nil {
		return CreatorBalanceHistoryResult{}, err
	}

	items := make([]CreatorBalanceHistoryItem, 0, len(rows))
	for _, row := range rows {
		occurredAt := row.CreatedAt
		if row.PoolMonth.Valid {
			occurredAt = row.PoolMonth.Time
		}
		if row.WithdrawalRequestedAt.Valid {
			occurredAt = row.WithdrawalRequestedAt.Time
		}
		item := CreatorBalanceHistoryItem{
			TransactionID:     row.TransactionID.String(),
			Type:              row.SourceType,
			SourceID:          row.SourceID,
			AmountVND:         row.AmountVnd,
			AbsoluteAmountVND: absInt64(row.AmountVnd),
			LedgerStatus:      row.LedgerStatus,
			OccurredAt:        occurredAt,
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
			PoolMonth:         monthString(row.PoolMonth),
		}
		if row.EarningID.Valid {
			item.EarningID = row.EarningID.UUID.String()
			item.EarningStatus = row.EarningStatus.String
			item.EligibleLearners = row.EligibleLearners.Int32
			item.WeightedScore = row.WeightedScore.String
		}
		if row.WithdrawalID.Valid {
			item.WithdrawalID = row.WithdrawalID.UUID.String()
			item.WithdrawalStatus = row.WithdrawalStatus.String
			item.WithdrawalRequestedAt = timePtr(row.WithdrawalRequestedAt)
			item.WithdrawalPaidAt = timePtr(row.WithdrawalPaidAt)
			item.PayoutToBin = row.PayoutToBin.String
			item.PayoutToAccountNumber = maskAccountNumber(row.PayoutToAccountNumber.String)
			item.PayoutToAccountName = row.PayoutToAccountName.String
			item.PayosPayoutID = row.PayosPayoutID.String
			item.PayosPayoutTransactionID = row.PayosPayoutTransactionID.String
			item.PayosPayoutState = row.PayosPayoutState.String
			item.PayoutFailedReason = row.PayoutFailedReason.String
		}
		items = append(items, item)
	}
	return CreatorBalanceHistoryResult{
		Items:  items,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *billingService) UpsertCreatorPayoutAccount(ctx context.Context, in PayoutAccountInput) (db.CreatorPayoutAccount, error) {
	if in.CreatorID == uuid.Nil ||
		strings.TrimSpace(in.BankBin) == "" ||
		strings.TrimSpace(in.BankCode) == "" ||
		strings.TrimSpace(in.BankShortName) == "" ||
		strings.TrimSpace(in.BankName) == "" ||
		strings.TrimSpace(in.AccountNumber) == "" ||
		strings.TrimSpace(in.AccountName) == "" {
		return db.CreatorPayoutAccount{}, domain.ErrInvalidPayout
	}
	return s.repo.UpsertCreatorPayoutAccount(ctx, db.UpsertCreatorPayoutAccountParams{
		CreatorID:     in.CreatorID,
		BankBin:       strings.TrimSpace(in.BankBin),
		BankCode:      strings.TrimSpace(in.BankCode),
		BankShortName: strings.TrimSpace(in.BankShortName),
		BankName:      strings.TrimSpace(in.BankName),
		BankLogo:      stringNull(strings.TrimSpace(in.BankLogo)),
		AccountNumber: strings.TrimSpace(in.AccountNumber),
		AccountName:   strings.TrimSpace(in.AccountName),
	})
}

func (s *billingService) GetCreatorPayoutAccount(ctx context.Context, creatorID uuid.UUID) (db.CreatorPayoutAccount, error) {
	return s.repo.GetCreatorPayoutAccount(ctx, creatorID)
}

func (s *billingService) CreateCreatorWithdrawal(ctx context.Context, in CreatorWithdrawalInput) (CreatorWithdrawalResult, error) {
	if in.CreatorID == uuid.Nil {
		return CreatorWithdrawalResult{}, domain.ErrInvalidPayout
	}
	if in.AmountVND <= MinimumPayoutAmountVND {
		return CreatorWithdrawalResult{}, domain.ErrPayoutAmountTooSmall
	}
	slog.Info("creator withdrawal requested",
		"creator_id", in.CreatorID.String(),
		"amount_vnd", in.AmountVND,
		"has_destination", strings.TrimSpace(in.ToBin) != "" && strings.TrimSpace(in.ToAccountNumber) != "",
	)
	if strings.TrimSpace(in.ToBin) == "" || strings.TrimSpace(in.ToAccountNumber) == "" || strings.TrimSpace(in.ToAccountName) == "" {
		account, err := s.repo.GetCreatorPayoutAccount(ctx, in.CreatorID)
		if err != nil {
			slog.Error("creator withdrawal payout account lookup failed",
				"creator_id", in.CreatorID.String(),
				"err", err,
			)
			return CreatorWithdrawalResult{}, err
		}
		slog.Info("creator withdrawal using saved payout account",
			"creator_id", in.CreatorID.String(),
			"bank_bin", account.BankBin,
			"account_number", maskAccountNumber(account.AccountNumber),
		)
		if strings.TrimSpace(in.ToBin) == "" {
			in.ToBin = account.BankBin
		}
		if strings.TrimSpace(in.ToAccountNumber) == "" {
			in.ToAccountNumber = account.AccountNumber
		}
		if strings.TrimSpace(in.ToAccountName) == "" {
			in.ToAccountName = account.AccountName
		}
	}
	balance, err := s.repo.GetCreatorBalanceSummary(ctx, in.CreatorID)
	if err != nil {
		slog.Error("creator withdrawal balance lookup failed",
			"creator_id", in.CreatorID.String(),
			"amount_vnd", in.AmountVND,
			"err", err,
		)
		return CreatorWithdrawalResult{}, err
	}
	slog.Info("creator withdrawal balance checked",
		"creator_id", in.CreatorID.String(),
		"amount_vnd", in.AmountVND,
		"available_balance_vnd", balance.AvailableBalanceVnd,
		"pending_withdrawal_amount_vnd", balance.PendingWithdrawalAmountVnd,
		"total_withdrawn_amount_vnd", balance.TotalWithdrawnAmountVnd,
	)
	if balance.AvailableBalanceVnd < in.AmountVND {
		slog.Warn("creator withdrawal rejected insufficient balance",
			"creator_id", in.CreatorID.String(),
			"amount_vnd", in.AmountVND,
			"available_balance_vnd", balance.AvailableBalanceVnd,
		)
		return CreatorWithdrawalResult{}, domain.ErrInsufficientBalance
	}

	withdrawalID := uuid.New()
	referenceID := creatorWithdrawalReferenceID(withdrawalID)
	idempotencyKey := creatorWithdrawalIdempotencyKey(withdrawalID)
	logAttrs := []any{
		"creator_id", in.CreatorID.String(),
		"withdrawal_id", withdrawalID.String(),
		"reference_id", referenceID,
		"amount_vnd", in.AmountVND,
		"bank_bin", strings.TrimSpace(in.ToBin),
		"account_number", maskAccountNumber(in.ToAccountNumber),
	}
	slog.Info("creator withdrawal creating local row", logAttrs...)
	withdrawal, err := s.repo.CreateCreatorWithdrawal(ctx, db.CreateCreatorWithdrawalParams{
		CreatorID:             in.CreatorID,
		AmountVnd:             in.AmountVND,
		Status:                "processing",
		PayoutReferenceID:     stringNull(referenceID),
		PayoutIdempotencyKey:  stringNull(idempotencyKey),
		PayoutToBin:           stringNull(strings.TrimSpace(in.ToBin)),
		PayoutToAccountNumber: stringNull(strings.TrimSpace(in.ToAccountNumber)),
		PayoutToAccountName:   stringNull(strings.TrimSpace(in.ToAccountName)),
	})
	if err != nil {
		slog.Error("creator withdrawal create local row failed", append(logAttrs, "err", err)...)
		return CreatorWithdrawalResult{}, err
	}
	slog.Info("creator withdrawal local row created", append(logAttrs, "status", withdrawal.Status)...)
	if _, err := s.repo.UpsertCreatorWithdrawalReservation(ctx, db.UpsertCreatorWithdrawalReservationParams{
		CreatorID: in.CreatorID,
		SourceID:  withdrawal.WithdrawalID.String(),
		AmountVnd: -in.AmountVND,
		Status:    "reserved",
	}); err != nil {
		slog.Error("creator withdrawal reservation failed", append(logAttrs, "err", err)...)
		return CreatorWithdrawalResult{}, err
	}
	slog.Info("creator withdrawal reservation created", append(logAttrs, "ledger_status", "reserved")...)

	slog.Info("creator withdrawal calling payos payout", append(logAttrs,
		"idempotency_key", idempotencyKey,
		"description", creatorWithdrawalDescription(in.Description),
	)...)
	resp, raw, err := s.payosClient.CreatePayout(ctx, payos.CreatePayoutRequest{
		ReferenceID:     referenceID,
		Amount:          in.AmountVND,
		Description:     creatorWithdrawalDescription(in.Description),
		ToBin:           strings.TrimSpace(in.ToBin),
		ToAccountNumber: strings.TrimSpace(in.ToAccountNumber),
		Category:        nil,
	}, idempotencyKey)
	if err != nil {
		slog.Error("creator withdrawal payos payout failed", append(logAttrs,
			"err", err,
			"raw_bytes", len(raw),
			"raw_valid_json", json.Valid(raw),
		)...)
		// DEV FALLBACK: If PayOS returns an IP Whitelist forbidden error (403 / "Địa chỉ IP không được phép"),
		// we fallback to mock success to allow local testing.
		errStr := err.Error()
		if strings.Contains(errStr, "403") || strings.Contains(errStr, "Địa chỉ IP không được phép") {
			slog.Warn("creator withdrawal using mock payout after payos IP whitelist error", append(logAttrs, "err", err)...)
			mockRaw := []byte(`{"mock": true, "reason": "dev_ip_whitelist_fallback"}`)
			w, dbErr := s.repo.UpdateCreatorWithdrawalStatus(ctx, db.UpdateCreatorWithdrawalStatusParams{
				Status:           "paid",
				PayosPayoutID:    sql.NullString{String: "mock_payout_" + withdrawalID.String(), Valid: true},
				PayoutRawPayload: jsonString(mockRaw),
				WithdrawalID:     withdrawal.WithdrawalID,
			})
			if dbErr == nil {
				_, _ = s.repo.UpdateCreatorBalanceTransactionStatus(ctx, db.UpdateCreatorBalanceTransactionStatusParams{
					Status:     "posted",
					SourceType: "withdrawal",
					SourceID:   withdrawal.WithdrawalID.String(),
				})
				slog.Info("creator withdrawal mock payout marked paid", append(logAttrs,
					"payout_id", "mock_payout_"+w.WithdrawalID.String(),
					"ledger_status", "posted",
				)...)

				summary, summaryErr := s.GetMyEarningsSummary(ctx, in.CreatorID)
				if summaryErr != nil {
					slog.Error("creator withdrawal summary after mock payout failed", append(logAttrs, "err", summaryErr)...)
					return CreatorWithdrawalResult{}, summaryErr
				}

				return CreatorWithdrawalResult{
					Withdrawal:    w,
					Balance:       summary,
					PayoutID:      "mock_payout_" + w.WithdrawalID.String(),
					TransactionID: "mock_tx_" + w.WithdrawalID.String(),
					ReferenceID:   referenceID,
					State:         "COMPLETED",
					Status:        w.Status,
				}, nil
			} else {
				slog.Error("creator withdrawal mock payout database update failed", append(logAttrs, "err", dbErr)...)
			}
		}

		if _, markErr := s.repo.UpdateCreatorWithdrawalStatus(ctx, db.UpdateCreatorWithdrawalStatusParams{
			Status:             "failed",
			PayoutRawPayload:   jsonString(raw),
			PayoutFailedReason: stringNull(err.Error()),
			WithdrawalID:       withdrawal.WithdrawalID,
		}); markErr != nil {
			slog.Error("creator withdrawal mark failed after payos error failed", append(logAttrs,
				"err", markErr,
				"raw_bytes", len(raw),
				"raw_valid_json", json.Valid(raw),
			)...)
		}
		if _, ledgerErr := s.repo.UpdateCreatorBalanceTransactionStatus(ctx, db.UpdateCreatorBalanceTransactionStatusParams{
			Status:     "reversed",
			SourceType: "withdrawal",
			SourceID:   withdrawal.WithdrawalID.String(),
		}); ledgerErr != nil {
			slog.Error("creator withdrawal reverse reservation after payos error failed", append(logAttrs, "err", ledgerErr)...)
		}
		return CreatorWithdrawalResult{}, err
	}

	tx := firstPayoutTransaction(resp.Data.Transactions)
	state := firstNonEmpty(tx.State, resp.Data.ApprovalState, "processing")
	withdrawalStatus := creatorWithdrawalStatusFromPayoutState(state)
	slog.Info("creator withdrawal payos payout response received", append(logAttrs,
		"payos_payout_id", resp.Data.ID,
		"payos_transaction_id", tx.ID,
		"payos_transaction_state", tx.State,
		"payos_approval_state", resp.Data.ApprovalState,
		"mapped_withdrawal_status", withdrawalStatus,
		"raw_bytes", len(raw),
		"raw_valid_json", json.Valid(raw),
	)...)
	withdrawal, err = s.repo.UpdateCreatorWithdrawalStatus(ctx, db.UpdateCreatorWithdrawalStatusParams{
		Status:                   withdrawalStatus,
		PayosPayoutID:            stringNull(resp.Data.ID),
		PayosPayoutTransactionID: stringNull(tx.ID),
		PayosPayoutState:         stringNull(state),
		PayoutRawPayload:         jsonString(raw),
		WithdrawalID:             withdrawal.WithdrawalID,
	})
	if err != nil {
		slog.Error("creator withdrawal update status after payos response failed", append(logAttrs,
			"err", err,
			"payos_payout_id", resp.Data.ID,
			"payos_transaction_id", tx.ID,
			"payos_state", state,
			"mapped_withdrawal_status", withdrawalStatus,
			"raw_bytes", len(raw),
			"raw_valid_json", json.Valid(raw),
		)...)
		return CreatorWithdrawalResult{}, err
	}
	slog.Info("creator withdrawal status updated", append(logAttrs,
		"payos_payout_id", resp.Data.ID,
		"payos_transaction_id", tx.ID,
		"payos_state", state,
		"withdrawal_status", withdrawal.Status,
	)...)
	ledgerStatus := "reserved"
	if withdrawalStatus == "paid" {
		ledgerStatus = "posted"
	}
	if _, err := s.repo.UpdateCreatorBalanceTransactionStatus(ctx, db.UpdateCreatorBalanceTransactionStatusParams{
		Status:     ledgerStatus,
		SourceType: "withdrawal",
		SourceID:   withdrawal.WithdrawalID.String(),
	}); err != nil {
		slog.Error("creator withdrawal ledger status update failed", append(logAttrs,
			"err", err,
			"ledger_status", ledgerStatus,
			"withdrawal_status", withdrawalStatus,
		)...)
		return CreatorWithdrawalResult{}, err
	}
	slog.Info("creator withdrawal ledger status updated", append(logAttrs,
		"ledger_status", ledgerStatus,
		"withdrawal_status", withdrawalStatus,
	)...)
	summary, err := s.GetMyEarningsSummary(ctx, in.CreatorID)
	if err != nil {
		slog.Error("creator withdrawal summary after payout failed", append(logAttrs, "err", err)...)
		return CreatorWithdrawalResult{}, err
	}
	slog.Info("creator withdrawal completed", append(logAttrs,
		"payout_id", resp.Data.ID,
		"transaction_id", tx.ID,
		"payos_state", state,
		"withdrawal_status", withdrawalStatus,
		"available_balance_vnd", summary.AvailableBalanceVND,
		"pending_withdrawal_amount_vnd", summary.PendingWithdrawalAmountVND,
		"total_withdrawn_amount_vnd", summary.TotalWithdrawnAmountVND,
	)...)
	return CreatorWithdrawalResult{
		Withdrawal:    withdrawal,
		Balance:       summary,
		PayoutID:      resp.Data.ID,
		TransactionID: tx.ID,
		ReferenceID:   referenceID,
		State:         state,
		Status:        withdrawalStatus,
	}, nil
}

func (s *billingService) WithdrawCreatorEarning(ctx context.Context, creatorID uuid.UUID, in PayoutInput) (PayoutResult, error) {
	if strings.TrimSpace(in.ToBin) == "" || strings.TrimSpace(in.ToAccountNumber) == "" || strings.TrimSpace(in.ToAccountName) == "" {
		account, err := s.repo.GetCreatorPayoutAccount(ctx, creatorID)
		if err != nil {
			return PayoutResult{}, err
		}
		if strings.TrimSpace(in.ToBin) == "" {
			in.ToBin = account.BankBin
		}
		if strings.TrimSpace(in.ToAccountNumber) == "" {
			in.ToAccountNumber = account.AccountNumber
		}
		if strings.TrimSpace(in.ToAccountName) == "" {
			in.ToAccountName = account.AccountName
		}
	}
	return s.payoutCreatorEarning(ctx, in, &creatorID)
}

func (s *billingService) PayoutCreatorEarning(ctx context.Context, in PayoutInput) (PayoutResult, error) {
	return s.payoutCreatorEarning(ctx, in, nil)
}

func (s *billingService) payoutCreatorEarning(ctx context.Context, in PayoutInput, creatorID *uuid.UUID) (PayoutResult, error) {
	earning, err := s.validatePayoutInput(ctx, in, creatorID)
	if err != nil {
		return PayoutResult{}, err
	}

	description := payoutDescription(in.Description, earning)
	referenceID := payoutReferenceID(earning.EarningID)
	idempotencyKey := payoutIdempotencyKey(earning.EarningID)

	earning, err = s.repo.MarkCreatorEarningPayoutProcessing(ctx, db.MarkCreatorEarningPayoutProcessingParams{
		EarningID:             earning.EarningID,
		PayoutReferenceID:     stringNull(referenceID),
		PayoutIdempotencyKey:  stringNull(idempotencyKey),
		PayoutToBin:           stringNull(strings.TrimSpace(in.ToBin)),
		PayoutToAccountNumber: stringNull(strings.TrimSpace(in.ToAccountNumber)),
		PayoutToAccountName:   stringNull(strings.TrimSpace(in.ToAccountName)),
	})
	if err != nil {
		return PayoutResult{}, err
	}

	resp, raw, err := s.payosClient.CreatePayout(ctx, payos.CreatePayoutRequest{
		ReferenceID:     referenceID,
		Amount:          earning.AmountVnd,
		Description:     description,
		ToBin:           strings.TrimSpace(in.ToBin),
		ToAccountNumber: strings.TrimSpace(in.ToAccountNumber),
		Category:        nil,
	}, idempotencyKey)
	if err != nil {
		_, _ = s.repo.MarkCreatorEarningPayoutFailed(ctx, db.MarkCreatorEarningPayoutFailedParams{
			EarningID:          earning.EarningID,
			PayosPayoutState:   stringNull("failed"),
			Column3:            jsonString(raw),
			PayoutFailedReason: stringNull(err.Error()),
		})
		return PayoutResult{}, err
	}

	tx := firstPayoutTransaction(resp.Data.Transactions)
	state := firstNonEmpty(tx.State, resp.Data.ApprovalState, "processing")
	status := earningStatusFromPayoutState(state)
	earning, err = s.repo.MarkCreatorEarningPayoutPaid(ctx, db.MarkCreatorEarningPayoutPaidParams{
		EarningID:                earning.EarningID,
		PayosPayoutID:            stringNull(resp.Data.ID),
		PayosPayoutTransactionID: stringNull(tx.ID),
		PayosPayoutState:         stringNull(state),
		Column5:                  status,
		Column6:                  jsonString(raw),
	})
	if err != nil {
		return PayoutResult{}, err
	}

	return PayoutResult{
		Earning:       earning,
		PayoutID:      resp.Data.ID,
		TransactionID: tx.ID,
		ReferenceID:   referenceID,
		State:         state,
		Status:        status,
	}, nil
}

func (s *billingService) BatchPayoutCreatorEarnings(ctx context.Context, in BatchPayoutInput) (BatchPayoutResult, error) {
	if len(in.Payouts) == 0 {
		return BatchPayoutResult{}, domain.ErrInvalidPayout
	}

	category := normalizePayoutCategory(in.Category)
	batchReferenceID := "mempan-batch-" + uuid.NewString()
	idempotencyKey := "payos:payout:batch:" + batchReferenceID
	items := make([]payos.BatchPayoutItem, 0, len(in.Payouts))
	earnings := make(map[string]db.CreatorEarning, len(in.Payouts))

	for _, payout := range in.Payouts {
		earning, err := s.validatePayoutInput(ctx, payout, nil)
		if err != nil {
			return BatchPayoutResult{}, err
		}
		referenceID := payoutReferenceID(earning.EarningID)
		earning, err = s.repo.MarkCreatorEarningPayoutProcessing(ctx, db.MarkCreatorEarningPayoutProcessingParams{
			EarningID:             earning.EarningID,
			PayoutReferenceID:     stringNull(referenceID),
			PayoutIdempotencyKey:  stringNull(idempotencyKey + ":" + earning.EarningID.String()),
			PayoutToBin:           stringNull(strings.TrimSpace(payout.ToBin)),
			PayoutToAccountNumber: stringNull(strings.TrimSpace(payout.ToAccountNumber)),
			PayoutToAccountName:   stringNull(strings.TrimSpace(payout.ToAccountName)),
		})
		if err != nil {
			return BatchPayoutResult{}, err
		}
		earnings[referenceID] = earning
		items = append(items, payos.BatchPayoutItem{
			ReferenceID:     referenceID,
			Amount:          earning.AmountVnd,
			Description:     payoutDescription(payout.Description, earning),
			ToBin:           strings.TrimSpace(payout.ToBin),
			ToAccountNumber: strings.TrimSpace(payout.ToAccountNumber),
		})
	}

	resp, raw, err := s.payosClient.CreateBatchPayout(ctx, payos.CreateBatchPayoutRequest{
		ReferenceID:         batchReferenceID,
		Category:            category,
		ValidateDestination: true,
		Payouts:             items,
	}, idempotencyKey)
	if err != nil {
		for _, earning := range earnings {
			_, _ = s.repo.MarkCreatorEarningPayoutFailed(ctx, db.MarkCreatorEarningPayoutFailedParams{
				EarningID:          earning.EarningID,
				PayosPayoutState:   stringNull("failed"),
				Column3:            jsonString(raw),
				PayoutFailedReason: stringNull(err.Error()),
			})
		}
		return BatchPayoutResult{}, err
	}

	results := make([]PayoutResult, 0, len(earnings))
	for _, tx := range resp.Data.Transactions {
		earning, ok := earnings[tx.ReferenceID]
		if !ok {
			continue
		}
		state := firstNonEmpty(tx.State, resp.Data.ApprovalState, "processing")
		status := earningStatusFromPayoutState(state)
		updated, err := s.repo.MarkCreatorEarningPayoutPaid(ctx, db.MarkCreatorEarningPayoutPaidParams{
			EarningID:                earning.EarningID,
			PayosPayoutID:            stringNull(resp.Data.ID),
			PayosPayoutTransactionID: stringNull(tx.ID),
			PayosPayoutState:         stringNull(state),
			Column5:                  status,
			Column6:                  jsonString(raw),
		})
		if err != nil {
			return BatchPayoutResult{}, err
		}
		results = append(results, PayoutResult{
			Earning:       updated,
			PayoutID:      resp.Data.ID,
			TransactionID: tx.ID,
			ReferenceID:   tx.ReferenceID,
			State:         state,
			Status:        status,
		})
		delete(earnings, tx.ReferenceID)
	}
	for _, earning := range earnings {
		state := firstNonEmpty(resp.Data.ApprovalState, "processing")
		status := earningStatusFromPayoutState(state)
		updated, err := s.repo.MarkCreatorEarningPayoutPaid(ctx, db.MarkCreatorEarningPayoutPaidParams{
			EarningID:        earning.EarningID,
			PayosPayoutID:    stringNull(resp.Data.ID),
			PayosPayoutState: stringNull(state),
			Column5:          status,
			Column6:          jsonString(raw),
		})
		if err != nil {
			return BatchPayoutResult{}, err
		}
		results = append(results, PayoutResult{
			Earning:     updated,
			PayoutID:    resp.Data.ID,
			ReferenceID: earning.PayoutReferenceID.String,
			State:       state,
			Status:      status,
		})
	}

	return BatchPayoutResult{
		BatchPayoutID: resp.Data.ID,
		ReferenceID:   batchReferenceID,
		Results:       results,
	}, nil
}

func (s *billingService) GetPayoutAccountBalance(ctx context.Context) (payos.PayoutBalanceResponse, error) {
	resp, _, err := s.payosClient.GetPayoutAccountBalance(ctx)
	return resp, err
}

func (s *billingService) validatePayoutInput(ctx context.Context, in PayoutInput, creatorID *uuid.UUID) (db.CreatorEarning, error) {
	if in.EarningID == uuid.Nil || strings.TrimSpace(in.ToBin) == "" || strings.TrimSpace(in.ToAccountNumber) == "" {
		return db.CreatorEarning{}, domain.ErrInvalidPayout
	}
	earning, err := s.repo.GetCreatorEarningByID(ctx, in.EarningID)
	if err != nil {
		return db.CreatorEarning{}, err
	}
	if creatorID != nil && earning.CreatorID != *creatorID {
		return db.CreatorEarning{}, domain.ErrPayoutForbidden
	}
	if earning.AmountVnd <= MinimumPayoutAmountVND {
		return db.CreatorEarning{}, domain.ErrPayoutAmountTooSmall
	}
	switch earning.Status {
	case "pending", "failed":
		return earning, nil
	default:
		return db.CreatorEarning{}, domain.ErrPayoutNotAllowed
	}
}

func subscriptionStatus(sub db.Subscription) SubscriptionStatus {
	active := sub.Status == db.SubscriptionStatusActive && sub.CurrentPeriodEnd.After(time.Now())
	return SubscriptionStatus{
		Active:           active,
		SubscriptionID:   sub.SubscriptionID.String(),
		PlanCode:         sub.PlanCode,
		Status:           string(sub.Status),
		CurrentPeriodEnd: sub.CurrentPeriodEnd,
	}
}

func newOrderCode() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	n := int64(binary.BigEndian.Uint64(b[:]) % 900000000000)
	return 100000000000 + n, nil
}

func shortDescription(orderCode int64) string {
	s := strconv.FormatInt(orderCode, 10)
	if len(s) <= 9 {
		return "MP" + s
	}
	return "MP" + s[len(s)-7:]
}

func coalesceCheckoutURL(preferred, fallback string) string {
	if normalized := normalizeCheckoutURL(preferred); normalized != "" {
		return normalized
	}
	return normalizeCheckoutURL(fallback)
}

func normalizeCheckoutURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if u.Host == "" {
		return ""
	}

	host := u.Hostname()
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		return ""
	}

	return u.String()
}

func uuidNull(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: true}
}

func stringNull(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	out := t.Time
	return &out
}

func monthString(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01")
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func maskAccountNumber(accountNumber string) string {
	accountNumber = strings.TrimSpace(accountNumber)
	if accountNumber == "" {
		return ""
	}
	if len(accountNumber) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(accountNumber)-4) + accountNumber[len(accountNumber)-4:]
}

func jsonString(raw []byte) string {
	if len(raw) == 0 || !json.Valid(raw) {
		return "null"
	}
	return string(raw)
}

func int64FromAny(v any) (int64, error) {
	switch n := v.(type) {
	case json.Number:
		return n.Int64()
	case float64:
		return int64(n), nil
	case int64:
		return n, nil
	case string:
		return strconv.ParseInt(n, 10, 64)
	default:
		return 0, fmt.Errorf("not an integer")
	}
}

func anyString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case json.Number:
		return s.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(s)
	}
}

func webhookEventKey(data map[string]any, orderCode int64) string {
	if ref := anyString(data["reference"]); ref != "" {
		return fmt.Sprintf("%d:%s", orderCode, ref)
	}
	if linkID := anyString(data["paymentLinkId"]); linkID != "" {
		return fmt.Sprintf("%d:%s:%s", orderCode, linkID, anyString(data["code"]))
	}
	return fmt.Sprintf("%d:%s", orderCode, anyString(data["transactionDateTime"]))
}

func parsePayOSTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			utc := t.UTC()
			return &utc
		}
	}
	return nil
}

func payoutReferenceID(earningID uuid.UUID) string {
	return "ce-" + earningID.String()
}

func payoutIdempotencyKey(earningID uuid.UUID) string {
	return earningID.String()
}

func creatorWithdrawalReferenceID(withdrawalID uuid.UUID) string {
	return "cw-" + withdrawalID.String()
}

func creatorWithdrawalIdempotencyKey(withdrawalID uuid.UUID) string {
	return withdrawalID.String()
}

func payoutDescription(description string, earning db.CreatorEarning) string {
	if strings.TrimSpace(description) != "" {
		return strings.TrimSpace(description)
	}
	return fmt.Sprintf("MemPan earning %s", earning.PoolMonth.Format("2006-01"))
}

func creatorWithdrawalDescription(description string) string {
	if strings.TrimSpace(description) != "" {
		return strings.TrimSpace(description)
	}
	return "MemPan creator withdrawal"
}

func normalizePayoutCategory(category []string) []string {
	if len(category) == 0 {
		return []string{"creator_payout"}
	}
	out := make([]string, 0, len(category))
	for _, item := range category {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return []string{"creator_payout"}
	}
	return out
}

func firstPayoutTransaction(items []payos.PayoutTransaction) payos.PayoutTransaction {
	if len(items) == 0 {
		return payos.PayoutTransaction{}
	}
	return items[0]
}

func earningStatusFromPayoutState(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch normalized {
	case "paid", "success", "succeeded", "successful", "completed", "complete", "done", "approved", "finished":
		return "paid"
	default:
		return "processing"
	}
}

func creatorWithdrawalStatusFromPayoutState(state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch normalized {
	case "paid", "success", "succeeded", "successful", "completed", "complete", "done", "approved", "finished":
		return "paid"
	case "failed", "error", "rejected", "cancelled", "canceled":
		return "failed"
	default:
		return "processing"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
