package payos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL           string
	clientID          string
	apiKey            string
	checksumKey       string
	payoutClientID    string
	payoutAPIKey      string
	payoutChecksumKey string
	httpClient        *http.Client
}

type Config struct {
	BaseURL           string
	ClientID          string
	APIKey            string
	ChecksumKey       string
	PayoutClientID    string
	PayoutAPIKey      string
	PayoutChecksumKey string
}

func NewClient(cfg Config) *Client {
	return &Client{
		baseURL:           strings.TrimRight(cfg.BaseURL, "/"),
		clientID:          cfg.ClientID,
		apiKey:            cfg.APIKey,
		checksumKey:       cfg.ChecksumKey,
		payoutClientID:    cfg.PayoutClientID,
		payoutAPIKey:      cfg.PayoutAPIKey,
		payoutChecksumKey: cfg.PayoutChecksumKey,
		httpClient:        &http.Client{Timeout: 15 * time.Second},
	}
}

type CreatePaymentLinkRequest struct {
	OrderCode   int64  `json:"orderCode"`
	Amount      int64  `json:"amount"`
	Description string `json:"description"`
	CancelURL   string `json:"cancelUrl"`
	ReturnURL   string `json:"returnUrl"`
	Signature   string `json:"signature"`
	BuyerName   string `json:"buyerName,omitempty"`
	BuyerEmail  string `json:"buyerEmail,omitempty"`
	BuyerPhone  string `json:"buyerPhone,omitempty"`
	ExpiredAt   int64  `json:"expiredAt,omitempty"`
	Items       []Item `json:"items,omitempty"`
}

type Item struct {
	Name     string `json:"name"`
	Quantity int32  `json:"quantity"`
	Price    int64  `json:"price"`
}

type CreatePaymentLinkResponse struct {
	Code      string          `json:"code"`
	Desc      string          `json:"desc"`
	Data      PaymentLinkData `json:"data"`
	Signature string          `json:"signature"`
}

type PaymentLinkData struct {
	PaymentLinkID string `json:"paymentLinkId"`
	OrderCode     int64  `json:"orderCode"`
	Amount        int64  `json:"amount"`
	Status        string `json:"status"`
	CheckoutURL   string `json:"checkoutUrl"`
	QRCode        string `json:"qrCode"`
}

type CreatePayoutRequest struct {
	ReferenceID     string   `json:"referenceId"`
	Amount          int64    `json:"amount"`
	Description     string   `json:"description"`
	ToBin           string   `json:"toBin"`
	ToAccountNumber string   `json:"toAccountNumber"`
	Category        []string `json:"category,omitempty"`
}

type BatchPayoutItem struct {
	ReferenceID     string `json:"referenceId"`
	Amount          int64  `json:"amount"`
	Description     string `json:"description"`
	ToBin           string `json:"toBin"`
	ToAccountNumber string `json:"toAccountNumber"`
}

type CreateBatchPayoutRequest struct {
	ReferenceID         string            `json:"referenceId"`
	Category            []string          `json:"category,omitempty"`
	ValidateDestination bool              `json:"validateDestination"`
	Payouts             []BatchPayoutItem `json:"payouts"`
}

type PayoutResponse struct {
	Code      string     `json:"code"`
	Desc      string     `json:"desc"`
	Data      PayoutData `json:"data"`
	Signature string     `json:"signature"`
}

type PayoutData struct {
	ID            string             `json:"id"`
	ReferenceID   string             `json:"referenceId"`
	ApprovalState string             `json:"approvalState"`
	Transactions  PayoutTransactions `json:"transactions"`
}

type PayoutTransaction struct {
	ID              string `json:"id"`
	ReferenceID     string `json:"referenceId"`
	Amount          int64  `json:"amount"`
	Description     string `json:"description"`
	ToBin           string `json:"toBin"`
	ToAccountNumber string `json:"toAccountNumber"`
	ToAccountName   string `json:"toAccountName"`
	State           string `json:"state"`
	ErrorMessage    string `json:"errorMessage"`
}

type PayoutTransactions []PayoutTransaction

func (p *PayoutTransactions) UnmarshalJSON(data []byte) error {
	var items []PayoutTransaction
	if err := json.Unmarshal(data, &items); err == nil {
		*p = items
		return nil
	}
	var itemMap map[string]PayoutTransaction
	if err := json.Unmarshal(data, &itemMap); err != nil {
		return err
	}
	items = make([]PayoutTransaction, 0, len(itemMap))
	for _, item := range itemMap {
		items = append(items, item)
	}
	*p = items
	return nil
}

type PayoutBalanceResponse struct {
	Code string            `json:"code"`
	Desc string            `json:"desc"`
	Data PayoutBalanceData `json:"data"`
}

type PayoutBalanceData struct {
	Balance json.RawMessage `json:"balance"`
}

// PaymentLinkInfoResponse is the response from GET /v2/payment-requests/{id}
type PaymentLinkInfoResponse struct {
	Code string              `json:"code"`
	Desc string              `json:"desc"`
	Data PaymentLinkInfoData `json:"data"`
}

type PaymentLinkInfoData struct {
	ID            string               `json:"id"`
	OrderCode     int64                `json:"orderCode"`
	Amount        int64                `json:"amount"`
	AmountPaid    int64                `json:"amountPaid"`
	AmountRemain  int64                `json:"amountRemaining"`
	Status        string               `json:"status"` // PAID, PENDING, PROCESSING, CANCELLED, EXPIRED
	CreatedAt     string               `json:"createdAt"`
	Transactions  []PaymentTransaction `json:"transactions"`
	CancelledAt   *string              `json:"cancelledAt"`
	CancellReason *string              `json:"cancellationReason"`
}

type PaymentTransaction struct {
	Reference             string `json:"reference"`
	Amount                int64  `json:"amount"`
	AccountNumber         string `json:"accountNumber"`
	Description           string `json:"description"`
	TransactionDateTime   string `json:"transactionDateTime"`
	VirtualAccountName    string `json:"virtualAccountName"`
	VirtualAccountNumber  string `json:"virtualAccountNumber"`
	CounterAccountBankID  string `json:"counterAccountBankId"`
	CounterAccountBankName string `json:"counterAccountBankName"`
	CounterAccountName    string `json:"counterAccountName"`
	CounterAccountNumber  string `json:"counterAccountNumber"`
}

func (c *Client) ChecksumKey() string {
	return c.checksumKey
}

// GetPaymentLinkInfo queries PayOS for the current status of a payment link.
func (c *Client) GetPaymentLinkInfo(ctx context.Context, orderCode int64) (PaymentLinkInfoResponse, []byte, error) {
	url := fmt.Sprintf("%s/v2/payment-requests/%d", c.baseURL, orderCode)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return PaymentLinkInfoResponse{}, nil, err
	}
	httpReq.Header.Set("x-client-id", c.clientID)
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PaymentLinkInfoResponse{}, nil, err
	}
	defer resp.Body.Close()

	var out PaymentLinkInfoResponse
	raw := new(bytes.Buffer)
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		return PaymentLinkInfoResponse{}, nil, err
	}
	if err := json.Unmarshal(raw.Bytes(), &out); err != nil {
		return PaymentLinkInfoResponse{}, raw.Bytes(), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PaymentLinkInfoResponse{}, raw.Bytes(), fmt.Errorf("payos get payment info failed: status=%d code=%s desc=%s", resp.StatusCode, out.Code, out.Desc)
	}
	return out, raw.Bytes(), nil
}

func (c *Client) SignCreatePaymentLink(req *CreatePaymentLinkRequest) error {
	sig, err := CreateSignature(map[string]any{
		"amount":      req.Amount,
		"cancelUrl":   req.CancelURL,
		"description": req.Description,
		"orderCode":   req.OrderCode,
		"returnUrl":   req.ReturnURL,
	}, c.checksumKey)
	if err != nil {
		return err
	}
	req.Signature = sig
	return nil
}

func (c *Client) CreatePaymentLink(ctx context.Context, req CreatePaymentLinkRequest) (CreatePaymentLinkResponse, []byte, error) {
	if err := c.SignCreatePaymentLink(&req); err != nil {
		return CreatePaymentLinkResponse{}, nil, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return CreatePaymentLinkResponse{}, nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/payment-requests", bytes.NewReader(body))
	if err != nil {
		return CreatePaymentLinkResponse{}, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-client-id", c.clientID)
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CreatePaymentLinkResponse{}, nil, err
	}
	defer resp.Body.Close()

	var out CreatePaymentLinkResponse
	raw := new(bytes.Buffer)
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		return CreatePaymentLinkResponse{}, nil, err
	}
	if err := json.Unmarshal(raw.Bytes(), &out); err != nil {
		return CreatePaymentLinkResponse{}, raw.Bytes(), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CreatePaymentLinkResponse{}, raw.Bytes(), fmt.Errorf("payos create payment link failed: status=%d code=%s desc=%s", resp.StatusCode, out.Code, out.Desc)
	}
	if out.Data.CheckoutURL == "" || out.Data.PaymentLinkID == "" {
		return CreatePaymentLinkResponse{}, raw.Bytes(), fmt.Errorf("payos create payment link returned incomplete data")
	}
	return out, raw.Bytes(), nil
}

func (c *Client) SignPayout(data map[string]any) (string, error) {
	return CreateSignature(data, c.payoutChecksumKey)
}

func (c *Client) CreatePayout(ctx context.Context, req CreatePayoutRequest, idempotencyKey string) (PayoutResponse, []byte, error) {
	signature, err := c.SignPayout(map[string]any{
		"amount":          req.Amount,
		"category":        req.Category,
		"description":     req.Description,
		"referenceId":     req.ReferenceID,
		"toAccountNumber": req.ToAccountNumber,
		"toBin":           req.ToBin,
	})
	if err != nil {
		return PayoutResponse{}, nil, err
	}
	return c.postPayout(ctx, "/v1/payouts", req, idempotencyKey, signature)
}

func (c *Client) CreateBatchPayout(ctx context.Context, req CreateBatchPayoutRequest, idempotencyKey string) (PayoutResponse, []byte, error) {
	signature, err := c.SignPayout(map[string]any{
		"category":            req.Category,
		"payouts":             req.Payouts,
		"referenceId":         req.ReferenceID,
		"validateDestination": req.ValidateDestination,
	})
	if err != nil {
		return PayoutResponse{}, nil, err
	}
	return c.postPayout(ctx, "/v1/payouts/batch", req, idempotencyKey, signature)
}

func (c *Client) GetPayoutAccountBalance(ctx context.Context) (PayoutBalanceResponse, []byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/payouts-account/balance", nil)
	if err != nil {
		return PayoutBalanceResponse{}, nil, err
	}
	httpReq.Header.Set("x-client-id", c.payoutClientID)
	httpReq.Header.Set("x-api-key", c.payoutAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PayoutBalanceResponse{}, nil, err
	}
	defer resp.Body.Close()

	var out PayoutBalanceResponse
	raw := new(bytes.Buffer)
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		return PayoutBalanceResponse{}, nil, err
	}
	if err := json.Unmarshal(raw.Bytes(), &out); err != nil {
		return PayoutBalanceResponse{}, raw.Bytes(), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PayoutBalanceResponse{}, raw.Bytes(), fmt.Errorf("payos get payout balance failed: status=%d code=%s desc=%s", resp.StatusCode, out.Code, out.Desc)
	}
	return out, raw.Bytes(), nil
}

func (c *Client) postPayout(ctx context.Context, path string, req any, idempotencyKey, signature string) (PayoutResponse, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return PayoutResponse{}, nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return PayoutResponse{}, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-client-id", c.payoutClientID)
	httpReq.Header.Set("x-api-key", c.payoutAPIKey)
	httpReq.Header.Set("x-idempotency-key", idempotencyKey)
	httpReq.Header.Set("x-signature", signature)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PayoutResponse{}, nil, err
	}
	defer resp.Body.Close()

	var out PayoutResponse
	raw := new(bytes.Buffer)
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		return PayoutResponse{}, nil, err
	}
	if err := json.Unmarshal(raw.Bytes(), &out); err != nil {
		return PayoutResponse{}, raw.Bytes(), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PayoutResponse{}, raw.Bytes(), fmt.Errorf("payos payout failed: status=%d code=%s desc=%s", resp.StatusCode, out.Code, out.Desc)
	}
	if out.Data.ID == "" {
		return PayoutResponse{}, raw.Bytes(), fmt.Errorf("payos payout returned incomplete data")
	}
	return out, raw.Bytes(), nil
}
