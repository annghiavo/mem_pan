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

type DeckCreatedEvent struct {
	DeckID    string    `json:"deck_id"`
	UserID    string    `json:"user_id"`
	DeckName  string    `json:"deck_name"`
	CreatedAt time.Time `json:"created_at"`
}

type DeckUpdatedEvent struct {
	DeckID   string `json:"deck_id"`
	UserID   string `json:"user_id"`
	DeckName string `json:"deck_name"`
}

type CardCreatedEvent struct {
	CardID    string    `json:"card_id"`
	DeckID    string    `json:"deck_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type EventPublisher interface {
	PublishDeckCreated(ctx context.Context, event DeckCreatedEvent) error
	PublishDeckUpdated(ctx context.Context, event DeckUpdatedEvent) error
	PublishCardCreated(ctx context.Context, event CardCreatedEvent) error
}

// noopPublisher logs events without sending them anywhere.
type noopPublisher struct{}

func NewNoopPublisher() EventPublisher { return &noopPublisher{} }

func (p *noopPublisher) PublishDeckCreated(_ context.Context, event DeckCreatedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] deck.created: %s", b)
	return nil
}

func (p *noopPublisher) PublishDeckUpdated(_ context.Context, event DeckUpdatedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] deck.updated: %s", b)
	return nil
}

func (p *noopPublisher) PublishCardCreated(_ context.Context, event CardCreatedEvent) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] card.created: %s", b)
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

func (p *httpPublisher) PublishDeckCreated(ctx context.Context, event DeckCreatedEvent) error {
	return p.publish(ctx, "deck.created", event)
}

func (p *httpPublisher) PublishDeckUpdated(ctx context.Context, event DeckUpdatedEvent) error {
	return p.publish(ctx, "deck.updated", event)
}

func (p *httpPublisher) PublishCardCreated(ctx context.Context, event CardCreatedEvent) error {
	return p.publish(ctx, "card.created", event)
}
