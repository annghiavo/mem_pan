package payos

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCreateSignatureSortsFields(t *testing.T) {
	got, err := CreateSignature(map[string]any{
		"returnUrl":   "https://example.com/return",
		"orderCode":   int64(123456),
		"description": "ORDER123",
		"cancelUrl":   "https://example.com/cancel",
		"amount":      int64(50000),
	}, "secret")
	if err != nil {
		t.Fatal(err)
	}

	const want = "b9c208495fda1d5748bae5dc8ebb1ff745aab6cfaa19a9808ffb0e29d808e0e3"
	if got != want {
		t.Fatalf("signature mismatch: got %s, want %s", got, want)
	}
}

func TestVerifySignatureRejectsMismatch(t *testing.T) {
	ok := VerifySignature(map[string]any{"amount": int64(50000)}, "bad-signature", "secret")
	if ok {
		t.Fatal("expected invalid signature")
	}
}

func TestPayoutUsesDisbursementCredentials(t *testing.T) {
	var gotClientID, gotAPIKey, gotSignature string
	client := NewClient(Config{
		BaseURL:           "https://payos.test",
		ClientID:          "payment-client",
		APIKey:            "payment-api",
		ChecksumKey:       "payment-checksum",
		PayoutClientID:    "payout-client",
		PayoutAPIKey:      "payout-api",
		PayoutChecksumKey: "payout-checksum",
	})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/payouts" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotClientID = r.Header.Get("x-client-id")
		gotAPIKey = r.Header.Get("x-api-key")
		gotSignature = r.Header.Get("x-signature")
		body, err := json.Marshal(PayoutResponse{
			Code: "00",
			Desc: "success",
			Data: PayoutData{ID: "payout_1", ApprovalState: "PROCESSING"},
		})
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	_, _, err := client.CreatePayout(
		t.Context(),
		CreatePayoutRequest{
			ReferenceID:     "ref_1",
			Amount:          100001,
			Description:     "creator payout",
			ToBin:           "970415",
			ToAccountNumber: "123456789",
			Category:        []string{"creator_payout"},
		},
		"idem_1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotClientID != "payout-client" {
		t.Fatalf("payout used wrong client id: got %q", gotClientID)
	}
	if gotAPIKey != "payout-api" {
		t.Fatalf("payout used wrong api key: got %q", gotAPIKey)
	}
	if gotSignature == "" {
		t.Fatal("expected payout signature")
	}
}

func TestPaymentLinkUsesPaymentCredentials(t *testing.T) {
	var gotClientID, gotAPIKey string
	client := NewClient(Config{
		BaseURL:           "https://payos.test",
		ClientID:          "payment-client",
		APIKey:            "payment-api",
		ChecksumKey:       "payment-checksum",
		PayoutClientID:    "payout-client",
		PayoutAPIKey:      "payout-api",
		PayoutChecksumKey: "payout-checksum",
	})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v2/payment-requests" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotClientID = r.Header.Get("x-client-id")
		gotAPIKey = r.Header.Get("x-api-key")
		body, err := json.Marshal(CreatePaymentLinkResponse{
			Code: "00",
			Desc: "success",
			Data: PaymentLinkData{
				PaymentLinkID: "plink_1",
				OrderCode:     123,
				CheckoutURL:   "https://pay.payos.vn/web/plink_1",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	_, _, err := client.CreatePaymentLink(t.Context(), CreatePaymentLinkRequest{
		OrderCode:   123,
		Amount:      49000,
		Description: "MP123",
		CancelURL:   "https://example.com/cancel",
		ReturnURL:   "https://example.com/return",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotClientID != "payment-client" {
		t.Fatalf("payment used wrong client id: got %q", gotClientID)
	}
	if gotAPIKey != "payment-api" {
		t.Fatalf("payment used wrong api key: got %q", gotAPIKey)
	}
	if strings.Contains(gotClientID+gotAPIKey, "payout") {
		t.Fatal("payment request used payout credentials")
	}
}
