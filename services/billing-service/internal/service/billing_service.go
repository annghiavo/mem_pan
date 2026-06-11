package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
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

	MinimumPayoutAmountVND = int64(100000)
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

type BatchPayoutInput struct {
	Payouts  []PayoutInput
	Category []string
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

type BillingService interface {
	CreateCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error)
	GetSubscriptionStatus(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error)
	CheckPlusAccess(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error)
	ProcessPayOSWebhook(ctx context.Context, in WebhookInput) error
	ConfirmPayment(ctx context.Context, userID uuid.UUID, orderCode int64) (ConfirmPaymentResult, error)
	ExpireSubscriptions(ctx context.Context) error

	GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error)
	GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error)
	GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error)
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
	if in.ReturnURL == "" {
		in.ReturnURL = s.defaultReturnURL
	}
	if in.CancelURL == "" {
		in.CancelURL = s.defaultCancelURL
	}

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
		RawPayload:        rawMessage(raw),
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

func (s *billingService) GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error) {
	return s.repo.GetMonthlyRevenuePools(ctx)
}

func (s *billingService) GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error) {
	return s.repo.GetCreatorEarningsByMonth(ctx, poolMonth)
}

func (s *billingService) GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error) {
	return s.repo.GetMyEarnings(ctx, creatorID)
}

func (s *billingService) PayoutCreatorEarning(ctx context.Context, in PayoutInput) (PayoutResult, error) {
	earning, err := s.validatePayoutInput(ctx, in)
	if err != nil {
		return PayoutResult{}, err
	}

	category := normalizePayoutCategory(in.Category)
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
		Category:        category,
	}, idempotencyKey)
	if err != nil {
		_, _ = s.repo.MarkCreatorEarningPayoutFailed(ctx, db.MarkCreatorEarningPayoutFailedParams{
			EarningID:          earning.EarningID,
			PayosPayoutState:   stringNull("failed"),
			PayoutRawPayload:   rawMessage(raw),
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
		Status:                   status,
		PayoutRawPayload:         rawMessage(raw),
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
		earning, err := s.validatePayoutInput(ctx, payout)
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
				PayoutRawPayload:   rawMessage(raw),
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
			Status:                   status,
			PayoutRawPayload:         rawMessage(raw),
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
			Status:           status,
			PayoutRawPayload: rawMessage(raw),
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

func (s *billingService) validatePayoutInput(ctx context.Context, in PayoutInput) (db.CreatorEarning, error) {
	if in.EarningID == uuid.Nil || strings.TrimSpace(in.ToBin) == "" || strings.TrimSpace(in.ToAccountNumber) == "" {
		return db.CreatorEarning{}, domain.ErrInvalidPayout
	}
	earning, err := s.repo.GetCreatorEarningByID(ctx, in.EarningID)
	if err != nil {
		return db.CreatorEarning{}, err
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

func uuidNull(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: true}
}

func stringNull(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func rawMessage(raw []byte) sql.NullString {
	return sql.NullString{
		String: string(raw),
		Valid:  len(raw) > 0,
	}
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
	return "payos:payout:" + earningID.String()
}

func payoutDescription(description string, earning db.CreatorEarning) string {
	if strings.TrimSpace(description) != "" {
		return strings.TrimSpace(description)
	}
	return fmt.Sprintf("MemPan earning %s", earning.PoolMonth.Format("2006-01"))
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
	case "paid", "success", "succeeded", "successful", "completed", "complete", "done":
		return "paid"
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
