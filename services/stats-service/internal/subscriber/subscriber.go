package subscriber

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"mem_pan/services/stats-service/internal/events"
)

// pushEnvelope is the JSON body Google Pub/Sub sends to a push endpoint.
type pushEnvelope struct {
	Message struct {
		Data        string            `json:"data"` // base64-encoded
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
		Attributes  map[string]string `json:"attributes"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// PushHandler implements http.Handler. Mount it at POST /internal/pubsub.
// Pub/Sub calls this endpoint for every message on the configured subscriptions.
type PushHandler struct {
	handler    *Handler
	pushSecret string // optional token validated via ?token= query param
}

func NewPushHandler(handler *Handler, pushSecret string) *PushHandler {
	return &PushHandler{handler: handler, pushSecret: pushSecret}
}

func (h *PushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate push secret when configured.
	if h.pushSecret != "" && r.URL.Query().Get("token") != h.pushSecret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB cap
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var push pushEnvelope
	if err := json.Unmarshal(body, &push); err != nil {
		// Malformed Pub/Sub envelope — ACK to stop infinite retries.
		log.Printf("[pubsub-push] malformed envelope: %v", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Pub/Sub encodes message.data as standard base64.
	raw, err := base64.StdEncoding.DecodeString(push.Message.Data)
	if err != nil {
		log.Printf("[pubsub-push] base64 decode error (msgID=%s): %v", push.Message.MessageID, err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var env events.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		log.Printf("[pubsub-push] malformed event envelope (msgID=%s): %v", push.Message.MessageID, err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.handler.Dispatch(r.Context(), env.EventType, env.Data); err != nil {
		// Return 5xx so Pub/Sub retries with exponential back-off.
		log.Printf("[pubsub-push] dispatch %q error (msgID=%s): %v", env.EventType, push.Message.MessageID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 204 = ACK — Pub/Sub will not redeliver this message.
	w.WriteHeader(http.StatusNoContent)
}
