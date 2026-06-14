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

	const want = "da0c6ff303c99e578de53ff2f5e99e5916f191e9852fe3a08ef299780e26993f"
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

func TestVerifySignatureIgnoresHexCase(t *testing.T) {
	ok := VerifySignature(map[string]any{"amount": int64(50000)}, "68AA0B394FC676F05824CE3FA7992B3419F32C12E856B6E78399B28149CD86BF", "secret")
	if !ok {
		t.Fatal("expected signature match regardless of hex case")
	}
}

func TestVerifyAnySignatureMatchesPayoutExample(t *testing.T) {
	data := map[string]any{
		"payouts": []any{
			map[string]any{
				"id":          "batch_8f9520b9341144f38b9f5fbfa317db8e",
				"referenceId": "payout_1753061728877",
				"transactions": []any{
					map[string]any{
						"id":                  "batch_txn_fdb348c0570a4cb99009da22f9504898",
						"referenceId":         "payout_1753061728877_0",
						"amount":              2000,
						"description":         "batch payout",
						"toBin":               "970422",
						"toAccountNumber":     "0123456789",
						"toAccountName":       "NGUYEN VAN A",
						"reference":           "103269845",
						"transactionDatetime": "2025-07-21T08:35:40+07:00",
						"errorMessage":        nil,
						"errorCode":           nil,
						"state":               "SUCCEEDED",
					},
					map[string]any{
						"id":                  "batch_txn_d94d371c079f4fc0ab7154e1576629d8",
						"referenceId":         "payout_1753061728877_1",
						"amount":              2000,
						"description":         "batch payout",
						"toBin":               "970422",
						"toAccountNumber":     "0123456789",
						"toAccountName":       "NGUYEN VAN A",
						"reference":           "103269846",
						"transactionDatetime": "2025-07-21T08:35:44+07:00",
						"errorMessage":        nil,
						"errorCode":           nil,
						"state":               "SUCCEEDED",
					},
					map[string]any{
						"id":                  "batch_txn_71eb922201f3442d93ec5a3c77347e9f",
						"referenceId":         "payout_1753061728877_2",
						"amount":              2000,
						"description":         "batch payout",
						"toBin":               "970422",
						"toAccountNumber":     "0123456789",
						"toAccountName":       "NGUYEN VAN A",
						"reference":           "103269847",
						"transactionDatetime": "2025-07-21T08:35:47+07:00",
						"errorMessage":        nil,
						"errorCode":           nil,
						"state":               "SUCCEEDED",
					},
				},
				"category":      []any{"salary"},
				"approvalState": "COMPLETED",
				"createdAt":     "2025-07-21T08:35:34+07:00",
			},
		},
		"pagination": map[string]any{
			"limit":   10,
			"offset":  0,
			"total":   1,
			"count":   1,
			"hasMore": false,
		},
	}
	if !VerifyAnySignature(data, "34d500c4e17feaad8fab528ac3ae089353e276ca9fb4c6654c06ffdfbd88cc5d", "6e91f59952acc8918c49c4a8e380136d66d1fbbf3375926840a8a7e434d4b325") {
		t.Fatal("expected payout example signature to verify")
	}
}

func TestPayoutUsesDisbursementCredentials(t *testing.T) {
	var gotClientID, gotAPIKey, gotSignature string
	responseData := PayoutData{ID: "payout_1", ApprovalState: "PROCESSING"}
	responseSignature, err := CreateSignature(map[string]any{
		"approvalState": responseData.ApprovalState,
		"id":            responseData.ID,
		"referenceId":   responseData.ReferenceID,
		"transactions":  responseData.Transactions,
	}, "payout-checksum")
	if err != nil {
		t.Fatal(err)
	}
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
			Code:      "00",
			Desc:      "success",
			Data:      responseData,
			Signature: responseSignature,
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

	_, _, err = client.CreatePayout(
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
