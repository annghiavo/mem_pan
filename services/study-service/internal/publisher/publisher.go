package publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type CardReviewedEvent struct {
	UserID         string    `json:"user_id"`
	CardID         string    `json:"card_id"`
	DeckID         string    `json:"deck_id"`
	Rating         int32     `json:"rating"`
	DurationMs     int64     `json:"duration_ms"`
	StateBefore    string    `json:"state_before"`
	StateAfter     string    `json:"state_after"`
	StabilityAfter float64   `json:"stability_after"`
	IsNewCard      bool      `json:"is_new_card"`
	ReviewTime     time.Time `json:"review_time"`
}

type EventPublisher interface {
	PublishCardReviewed(ctx context.Context, event CardReviewedEvent) error
}

// noopPublisher logs events without sending them anywhere.
type noopPublisher struct{}

func NewNoopPublisher() EventPublisher { return &noopPublisher{} }

func (p *noopPublisher) PublishCardReviewed(_ context.Context, event CardReviewedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] card.reviewed: %s", b)
	return nil
}

type envelope struct {
	EventType string `json:"event_type"`
	Data      []byte `json:"data"`
}

type httpPublisher struct {
	endpoint string
	client   *http.Client
}

func NewPubSubPublisher(projectID, topicID string) EventPublisher {
	host := os.Getenv("PUBSUB_EMULATOR_HOST")
	base := "https://pubsub.googleapis.com"
	if host != "" {
		base = "http://" + host
	}
	return &httpPublisher{
		endpoint: fmt.Sprintf("%s/v1/projects/%s/topics/%s:publish", base, projectID, topicID),
		client:   &http.Client{},
	}
}

func (p *httpPublisher) publish(ctx context.Context, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env, err := json.Marshal(envelope{EventType: eventType, Data: data})
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"messages": []map[string]any{
			{"data": base64.StdEncoding.EncodeToString(env)},
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("pubsub: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (p *httpPublisher) PublishCardReviewed(ctx context.Context, event CardReviewedEvent) error {
	return p.publish(ctx, "card.reviewed", event)
}
