package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/billing-service/internal/authclient"
	"mem_pan/services/billing-service/internal/domain"
	"mem_pan/services/billing-service/internal/service"
)

type Handler struct {
	billingSvc service.BillingService
	authClient authclient.Client
}

func NewHandler(billingSvc service.BillingService, authClient authclient.Client) *Handler {
	return &Handler{billingSvc: billingSvc, authClient: authClient}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/billing/checkout", h.handleCheckout)
	mux.HandleFunc("/v1/billing/confirm", h.handleConfirmPayment)
	mux.HandleFunc("/v1/billing/subscription/me", h.handleSubscriptionMe)
	mux.HandleFunc("/v1/billing/webhooks/payos", h.handlePayOSWebhook)
	mux.HandleFunc("/v1/admin/revenue/pools", h.handleAdminGetPools)
	mux.HandleFunc("/v1/admin/revenue/payouts", h.handleAdminGetPayouts)
	mux.HandleFunc("/v1/admin/revenue/payouts/pay", h.handleAdminPayPayout)
	mux.HandleFunc("/v1/admin/revenue/payouts/batch", h.handleAdminBatchPayPayouts)
	mux.HandleFunc("/v1/admin/revenue/payouts/balance", h.handleAdminPayoutBalance)
	mux.HandleFunc("/v1/admin/revenue/payouts/mark-paid", h.handleAdminMarkPaid)
	mux.HandleFunc("/v1/creators/me/earnings", h.handleGetMyEarnings)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

type checkoutRequest struct {
	PlanCode  string `json:"plan_code"`
	ReturnURL string `json:"return_url"`
	CancelURL string `json:"cancel_url"`
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
	case errors.Is(err, domain.ErrPaymentNotFound), errors.Is(err, domain.ErrSubscriptionNotFound), errors.Is(err, domain.ErrEarningNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrPayoutNotAllowed):
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
