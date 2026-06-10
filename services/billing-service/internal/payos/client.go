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
	baseURL     string
	clientID    string
	apiKey      string
	checksumKey string
	httpClient  *http.Client
}

func NewClient(baseURL, clientID, apiKey, checksumKey string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		clientID:    clientID,
		apiKey:      apiKey,
		checksumKey: checksumKey,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
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

func (c *Client) ChecksumKey() string {
	return c.checksumKey
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
