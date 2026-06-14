package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/billing-service/internal/authclient"
	"mem_pan/services/billing-service/internal/db"
	"mem_pan/services/billing-service/internal/domain"
	"mem_pan/services/billing-service/internal/service"
)

type Handler struct {
	billingSvc service.BillingService
	authClient authclient.Client
	bankCache  bankCache
}

type bankCache struct {
	mu        sync.Mutex
	expiresAt time.Time
	banks     []bankResponse
}

func NewHandler(billingSvc service.BillingService, authClient authclient.Client) *Handler {
	return &Handler{billingSvc: billingSvc, authClient: authClient}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/billing/checkout", h.handleCheckout)
	mux.HandleFunc("/v1/billing/confirm", h.handleConfirmPayment)
	mux.HandleFunc("/v1/billing/banks", h.handleBanks)
	mux.HandleFunc("/v1/billing/subscription/me", h.handleSubscriptionMe)
	mux.HandleFunc("/v1/billing/webhooks/payos", h.handlePayOSWebhook)
	mux.HandleFunc("/v1/admin/revenue/pools", h.handleAdminGetPools)
	mux.HandleFunc("/v1/admin/revenue/payouts", h.handleAdminGetPayouts)
	mux.HandleFunc("/v1/admin/revenue/payouts/pay", h.handleAdminPayPayout)
	mux.HandleFunc("/v1/admin/revenue/payouts/batch", h.handleAdminBatchPayPayouts)
	mux.HandleFunc("/v1/admin/revenue/payouts/balance", h.handleAdminPayoutBalance)
	mux.HandleFunc("/v1/admin/revenue/payouts/mark-paid", h.handleAdminMarkPaid)
	mux.HandleFunc("/v1/creators/me/earnings/summary", h.handleGetMyEarningsSummary)
	mux.HandleFunc("/v1/creators/me/earnings", h.handleGetMyEarnings)
	mux.HandleFunc("/v1/creators/me/balance-history", h.handleGetMyBalanceHistory)
	mux.HandleFunc("/v1/creators/me/payout-account", h.handleMyPayoutAccount)
	mux.HandleFunc("/v1/creators/me/withdrawals", h.handleCreateMyWithdrawal)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

type checkoutRequest struct {
	PlanCode  string `json:"plan_code"`
	ReturnURL string `json:"return_url"`
	CancelURL string `json:"cancel_url"`
}

type vietQRBanksResponse struct {
	Data []vietQRBank `json:"data"`
}

type vietQRBank struct {
	Bin               string `json:"bin"`
	Code              string `json:"code"`
	ShortName         string `json:"shortName"`
	Name              string `json:"name"`
	Logo              string `json:"logo"`
	TransferSupported int    `json:"transferSupported"`
}

type bankResponse struct {
	Bin               string `json:"bin"`
	Code              string `json:"code"`
	ShortName         string `json:"short_name"`
	Name              string `json:"name"`
	Logo              string `json:"logo"`
	TransferSupported bool   `json:"transfer_supported"`
}

func (h *Handler) handleBanks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	banks, err := h.getBanks(r)
	if err != nil {
		slog.Error("fetch banks", "err", err.Error())
		writeError(w, http.StatusBadGateway, "unable to load bank list")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": banks})
}

func (h *Handler) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	result, err := h.billingSvc.CreateCheckout(r.Context(), service.CheckoutInput{
		UserID:    userID,
		PlanCode:  req.PlanCode,
		ReturnURL: req.ReturnURL,
		CancelURL: req.CancelURL,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) handleSubscriptionMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	result, err := h.billingSvc.GetSubscriptionStatus(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type confirmPaymentRequest struct {
	OrderCode int64 `json:"order_code"`
}

func (h *Handler) handleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req confirmPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.OrderCode == 0 {
		writeError(w, http.StatusBadRequest, "order_code is required")
		return
	}
	result, err := h.billingSvc.ConfirmPayment(r.Context(), userID, req.OrderCode)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePayOSWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var body service.PayOSWebhook
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if err := h.billingSvc.ProcessPayOSWebhook(r.Context(), service.WebhookInput{
		RawPayload: raw,
		Body:       body,
	}); err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleAdminGetPools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := h.authorizeAdmin(w, r); !ok {
		return
	}
	pools, err := h.billingSvc.GetMonthlyRevenuePools(r.Context())
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pools)
}

func (h *Handler) handleAdminGetPayouts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := h.authorizeAdmin(w, r); !ok {
		return
	}
	monthStr := r.URL.Query().Get("month")
	if monthStr == "" {
		writeError(w, http.StatusBadRequest, "missing month parameter (YYYY-MM-DD)")
		return
	}
	t, err := time.Parse("2006-01-02", monthStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid month format")
		return
	}
	payouts, err := h.billingSvc.GetCreatorEarningsByMonth(r.Context(), t)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payouts)
}

type payoutRequest struct {
	EarningID       string   `json:"earning_id"`
	AmountVND       int64    `json:"amount_vnd"`
	ToBin           string   `json:"to_bin"`
	ToAccountNumber string   `json:"to_account_number"`
	ToAccountName   string   `json:"to_account_name"`
	Description     string   `json:"description"`
	Category        []string `json:"category"`
}

type batchPayoutRequest struct {
	Payouts  []payoutRequest `json:"payouts"`
	Category []string        `json:"category"`
}

type payoutAccountRequest struct {
	BankBin       string `json:"bank_bin"`
	BankCode      string `json:"bank_code"`
	BankShortName string `json:"bank_short_name"`
	BankName      string `json:"bank_name"`
	BankLogo      string `json:"bank_logo"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type payoutAccountResponse struct {
	CreatorID     string     `json:"creator_id"`
	BankBin       string     `json:"bank_bin"`
	BankCode      string     `json:"bank_code"`
	BankShortName string     `json:"bank_short_name"`
	BankName      string     `json:"bank_name"`
	BankLogo      string     `json:"bank_logo,omitempty"`
	AccountNumber string     `json:"account_number"`
	AccountName   string     `json:"account_name"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (h *Handler) handleAdminPayPayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := h.authorizeAdmin(w, r); !ok {
		return
	}
	var req payoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	input, ok := payoutInputFromRequest(w, req)
	if !ok {
		return
	}
	result, err := h.billingSvc.PayoutCreatorEarning(r.Context(), input)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleAdminBatchPayPayouts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := h.authorizeAdmin(w, r); !ok {
		return
	}
	var req batchPayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	inputs := make([]service.PayoutInput, 0, len(req.Payouts))
	for _, payout := range req.Payouts {
		input, ok := payoutInputFromRequest(w, payout)
		if !ok {
			return
		}
		inputs = append(inputs, input)
	}
	result, err := h.billingSvc.BatchPayoutCreatorEarnings(r.Context(), service.BatchPayoutInput{
		Payouts:  inputs,
		Category: req.Category,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleAdminPayoutBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := h.authorizeAdmin(w, r); !ok {
		return
	}
	result, err := h.billingSvc.GetPayoutAccountBalance(r.Context())
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleAdminMarkPaid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := h.authorizeAdmin(w, r); !ok {
		return
	}
	var req payoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	input, ok := payoutInputFromRequest(w, req)
	if !ok {
		return
	}
	result, err := h.billingSvc.PayoutCreatorEarning(r.Context(), input)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGetMyEarnings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	earnings, err := h.billingSvc.GetMyEarnings(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, earnings)
}

func (h *Handler) handleGetMyEarningsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	summary, err := h.billingSvc.GetMyEarningsSummary(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) handleGetMyBalanceHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	limit, err := int32Query(r, "limit", 80)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid limit")
		return
	}
	offset, err := int32Query(r, "offset", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid offset")
		return
	}
	result, err := h.billingSvc.ListMyBalanceHistory(r.Context(), userID, limit, offset)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleMyPayoutAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		account, err := h.billingSvc.GetCreatorPayoutAccount(r.Context(), userID)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, payoutAccountToResponse(account))
	case http.MethodPut:
		var req payoutAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		account, err := h.billingSvc.UpsertCreatorPayoutAccount(r.Context(), service.PayoutAccountInput{
			CreatorID:     userID,
			BankBin:       req.BankBin,
			BankCode:      req.BankCode,
			BankShortName: req.BankShortName,
			BankName:      req.BankName,
			BankLogo:      req.BankLogo,
			AccountNumber: req.AccountNumber,
			AccountName:   req.AccountName,
		})
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, payoutAccountToResponse(account))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleCreateMyWithdrawal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req payoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	result, err := h.billingSvc.CreateCreatorWithdrawal(r.Context(), service.CreatorWithdrawalInput{
		CreatorID:       userID,
		AmountVND:       req.AmountVND,
		ToBin:           req.ToBin,
		ToAccountNumber: req.ToAccountNumber,
		ToAccountName:   req.ToAccountName,
		Description:     req.Description,
		Category:        req.Category,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	payload, ok := h.authorizePayload(w, r)
	if !ok {
		return uuid.Nil, false
	}
	return payload.UserID, true
}

func (h *Handler) authorizePayload(w http.ResponseWriter, r *http.Request) (*authclient.Payload, bool) {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
		return nil, false
	}
	payload, err := h.authClient.VerifyToken(r.Context(), fields[1])
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired access token")
		return nil, false
	}
	return payload, true
}

func (h *Handler) authorizeAdmin(w http.ResponseWriter, r *http.Request) (*authclient.Payload, bool) {
	payload, ok := h.authorizePayload(w, r)
	if !ok {
		return nil, false
	}
	if payload.Role != "admin" && payload.Role != "moderator" {
		writeError(w, http.StatusForbidden, "admin access required")
		return nil, false
	}
	return payload, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPlan), errors.Is(err, domain.ErrInvalidWebhook), errors.Is(err, domain.ErrAmountMismatch), errors.Is(err, domain.ErrInvalidPayout), errors.Is(err, domain.ErrPayoutAmountTooSmall):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrPaymentNotFound), errors.Is(err, domain.ErrSubscriptionNotFound), errors.Is(err, domain.ErrEarningNotFound), errors.Is(err, domain.ErrPayoutAccountNotFound), errors.Is(err, domain.ErrWithdrawalNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrPayoutForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrPayoutNotAllowed):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInsufficientBalance):
		writeError(w, http.StatusConflict, err.Error())
	default:
		slog.Error("billing http error", "err", err.Error())
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func payoutInputFromRequest(w http.ResponseWriter, req payoutRequest) (service.PayoutInput, bool) {
	earningUUID, err := uuid.Parse(req.EarningID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid earning_id")
		return service.PayoutInput{}, false
	}
	return service.PayoutInput{
		EarningID:       earningUUID,
		ToBin:           req.ToBin,
		ToAccountNumber: req.ToAccountNumber,
		ToAccountName:   req.ToAccountName,
		Description:     req.Description,
		Category:        req.Category,
	}, true
}

func int32Query(r *http.Request, key string, fallback int32) (int32, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(value), nil
}

func (h *Handler) getBanks(r *http.Request) ([]bankResponse, error) {
	now := time.Now()
	h.bankCache.mu.Lock()
	if now.Before(h.bankCache.expiresAt) && len(h.bankCache.banks) > 0 {
		banks := append([]bankResponse(nil), h.bankCache.banks...)
		h.bankCache.mu.Unlock()
		return banks, nil
	}
	h.bankCache.mu.Unlock()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://api.vietqr.io/v2/banks", nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("vietqr returned non-2xx status")
	}

	var payload vietQRBanksResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	banks := make([]bankResponse, 0, len(payload.Data))
	for _, bank := range payload.Data {
		if bank.TransferSupported != 1 {
			continue
		}
		out := bankResponse{
			Bin:               strings.TrimSpace(bank.Bin),
			Code:              strings.TrimSpace(bank.Code),
			ShortName:         strings.TrimSpace(bank.ShortName),
			Name:              strings.TrimSpace(bank.Name),
			Logo:              strings.TrimSpace(bank.Logo),
			TransferSupported: true,
		}
		if out.Bin == "" || out.Code == "" || out.ShortName == "" || out.Name == "" {
			continue
		}
		banks = append(banks, out)
	}

	h.bankCache.mu.Lock()
	h.bankCache.banks = append([]bankResponse(nil), banks...)
	h.bankCache.expiresAt = now.Add(24 * time.Hour)
	h.bankCache.mu.Unlock()
	return banks, nil
}

func payoutAccountToResponse(account db.CreatorPayoutAccount) payoutAccountResponse {
	var verifiedAt *time.Time
	if account.VerifiedAt.Valid {
		t := account.VerifiedAt.Time
		verifiedAt = &t
	}
	return payoutAccountResponse{
		CreatorID:     account.CreatorID.String(),
		BankBin:       account.BankBin,
		BankCode:      account.BankCode,
		BankShortName: account.BankShortName,
		BankName:      account.BankName,
		BankLogo:      account.BankLogo.String,
		AccountNumber: account.AccountNumber,
		AccountName:   account.AccountName,
		VerifiedAt:    verifiedAt,
		CreatedAt:     account.CreatedAt,
		UpdatedAt:     account.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
