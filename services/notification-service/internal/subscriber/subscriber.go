package subscriber

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"mem_pan/services/notification-service/internal/events"
)

type pushEnvelope struct {
	Message struct {
		Data        string            `json:"data"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
		Attributes  map[string]string `json:"attributes"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// PushHandler implements http.Handler. Mount it at POST /internal/pubsub.
type PushHandler struct {
	handler    *Handler
	pushSecret string
}

func NewPushHandler(handler *Handler, pushSecret string) *PushHandler {
	return &PushHandler{handler: handler, pushSecret: pushSecret}
}

func (h *PushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.pushSecret != "" && r.URL.Query().Get("token") != h.pushSecret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var push pushEnvelope
	if err := json.Unmarshal(body, &push); err != nil {
		log.Printf("[pubsub-push] malformed envelope: %v", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

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
		log.Printf("[pubsub-push] dispatch %q error (msgID=%s): %v", env.EventType, push.Message.MessageID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
