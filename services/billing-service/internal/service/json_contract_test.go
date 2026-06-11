package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"mem_pan/services/billing-service/internal/db"
)

func TestSubscriptionStatusJSONUsesAPIFieldNames(t *testing.T) {
	periodEnd := time.Date(2026, 7, 11, 18, 25, 56, 0, time.UTC)
	body, err := json.Marshal(SubscriptionStatus{
		Active:           true,
		SubscriptionID:   "sub_123",
		PlanCode:         PlanPlusMonthly,
		Status:           "active",
		CurrentPeriodEnd: periodEnd,
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"active", "subscription_id", "plan_code", "status", "current_period_end"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected JSON key %q in %s", key, string(body))
		}
	}
	for _, key := range []string{"Active", "SubscriptionID", "PlanCode", "Status", "CurrentPeriodEnd"} {
		if _, ok := got[key]; ok {
			t.Fatalf("unexpected Go-style JSON key %q in %s", key, string(body))
		}
	}
}

func TestCheckoutResultJSONUsesAPIFieldNames(t *testing.T) {
	body, err := json.Marshal(CheckoutResult{
		SubscriptionID: "sub_123",
		TransactionID:  "tx_123",
		OrderCode:      123456789,
		PaymentLinkID:  "plink_123",
		CheckoutURL:    "https://pay.example/checkout",
		AmountVND:      25000,
		Status:         "pending",
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"subscription_id", "transaction_id", "order_code", "payment_link_id", "checkout_url", "amount_vnd", "status"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected JSON key %q in %s", key, string(body))
		}
	}
	for _, key := range []string{"SubscriptionID", "TransactionID", "OrderCode", "PaymentLinkID", "CheckoutURL", "AmountVND", "Status"} {
		if _, ok := got[key]; ok {
			t.Fatalf("unexpected Go-style JSON key %q in %s", key, string(body))
		}
	}
}

func TestCalculateActivationPeriodStacksAfterExistingActiveSubscription(t *testing.T) {
	currentSubscriptionID := uuid.New()
	existingSubscriptionID := uuid.New()
	paidAt := time.Date(2026, 6, 11, 18, 25, 56, 0, time.UTC)
	existingEnd := time.Date(2026, 7, 11, 16, 55, 0, 0, time.UTC)

	start, end := calculateActivationPeriod(currentSubscriptionID, paidAt, 30*24*time.Hour, db.Subscription{
		SubscriptionID:   existingSubscriptionID,
		CurrentPeriodEnd: existingEnd,
	}, true)

	if !start.Equal(existingEnd) {
		t.Fatalf("expected stacked period to start at existing end %s, got %s", existingEnd, start)
	}
	wantEnd := existingEnd.Add(30 * 24 * time.Hour)
	if !end.Equal(wantEnd) {
		t.Fatalf("expected stacked period to end at %s, got %s", wantEnd, end)
	}
}

func TestCalculateActivationPeriodDoesNotExtendSameSubscriptionOnRetry(t *testing.T) {
	subscriptionID := uuid.New()
	paidAt := time.Date(2026, 6, 11, 18, 25, 56, 0, time.UTC)
	existingEnd := time.Date(2026, 7, 11, 18, 25, 56, 0, time.UTC)

	start, end := calculateActivationPeriod(subscriptionID, paidAt, 30*24*time.Hour, db.Subscription{
		SubscriptionID:   subscriptionID,
		CurrentPeriodEnd: existingEnd,
	}, true)

	if !start.Equal(paidAt) {
		t.Fatalf("expected same subscription retry to keep original paid start %s, got %s", paidAt, start)
	}
	wantEnd := paidAt.Add(30 * 24 * time.Hour)
	if !end.Equal(wantEnd) {
		t.Fatalf("expected same subscription retry to end at %s, got %s", wantEnd, end)
	}
}
