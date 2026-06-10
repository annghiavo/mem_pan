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
	mux.HandleFunc("/v1/billing/subscription/me", h.handleSubscriptionMe)
	mux.HandleFunc("/v1/billing/webhooks/payos", h.handlePayOSWebhook)
	mux.HandleFunc("/v1/admin/revenue/pools", h.handleAdminGetPools)
	mux.HandleFunc("/v1/admin/revenue/payouts", h.handleAdminGetPayouts)
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
	// TODO: verify admin role from auth client
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
	// TODO: verify admin role
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

func (h *Handler) handleAdminMarkPaid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// TODO: verify admin role
	var req struct {
		EarningID string `json:"earning_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	earningUUID, err := uuid.Parse(req.EarningID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid earning_id")
		return
	}
	earning, err := h.billingSvc.MarkCreatorEarningPaid(r.Context(), earningUUID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, earning)
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
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
		return uuid.Nil, false
	}
	payload, err := h.authClient.VerifyToken(r.Context(), fields[1])
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired access token")
		return uuid.Nil, false
	}
	return payload.UserID, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPlan), errors.Is(err, domain.ErrInvalidWebhook), errors.Is(err, domain.ErrAmountMismatch):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrPaymentNotFound), errors.Is(err, domain.ErrSubscriptionNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		slog.Error("billing http error", "err", err.Error())
		writeError(w, http.StatusInternalServerError, "internal server error")
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
