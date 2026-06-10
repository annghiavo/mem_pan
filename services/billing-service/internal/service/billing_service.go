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
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"mem_pan/services/billing-service/internal/db"
	"mem_pan/services/billing-service/internal/domain"
	"mem_pan/services/billing-service/internal/payos"
	"mem_pan/services/billing-service/internal/repository"
)

const (
	PlanPlusMonthly = "plus_monthly"
	PlanPlusYearly  = "plus_yearly"
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
	SubscriptionID string
	TransactionID  string
	OrderCode      int64
	PaymentLinkID  string
	CheckoutURL    string
	AmountVND      int64
	Status         string
}

type SubscriptionStatus struct {
	Active           bool
	SubscriptionID   string
	PlanCode         string
	Status           string
	CurrentPeriodEnd time.Time
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

type BillingService interface {
	CreateCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error)
	GetSubscriptionStatus(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error)
	CheckPlusAccess(ctx context.Context, userID uuid.UUID) (SubscriptionStatus, error)
	ProcessPayOSWebhook(ctx context.Context, in WebhookInput) error
	ExpireSubscriptions(ctx context.Context) error

	GetMonthlyRevenuePools(ctx context.Context) ([]db.MonthlyRevenuePool, error)
	GetCreatorEarningsByMonth(ctx context.Context, poolMonth time.Time) ([]db.CreatorEarning, error)
	GetMyEarnings(ctx context.Context, creatorID uuid.UUID) ([]db.CreatorEarning, error)
	MarkCreatorEarningPaid(ctx context.Context, earningID uuid.UUID) (db.CreatorEarning, error)
}

type billingService struct {
	repo             repository.BillingRepository
	payosClient      *payos.Client
	plans            map[string]Plan
	defaultReturnURL string
	defaultCancelURL string
	now              func() time.Time
}

func New(repo repository.BillingRepository, payosClient *payos.Client, monthlyAmount, yearlyAmount int64, returnURL, cancelURL string) BillingService {
	return &billingService{
		repo:        repo,
		payosClient: payosClient,
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
	sub, err := s.repo.GetLatestSubscriptionForUser(ctx, userID)
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
			_, err = s.repo.ActivateSubscription(ctx, sub.SubscriptionID, paidTime, paidTime.Add(plan.Duration))
			return err
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

func (s *billingService) MarkCreatorEarningPaid(ctx context.Context, earningID uuid.UUID) (db.CreatorEarning, error) {
	return s.repo.MarkCreatorEarningPaid(ctx, earningID)
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

func rawMessage(raw []byte) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{
		RawMessage: json.RawMessage(raw),
		Valid:      len(raw) > 0,
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
