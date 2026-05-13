package publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

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

func (p *httpPublisher) PublishUserRegistered(ctx context.Context, event UserRegisteredEvent) error {
	return p.publish(ctx, "user.registered", event)
}

func (p *httpPublisher) PublishEmailVerificationRequested(ctx context.Context, event EmailVerificationRequestedEvent) error {
	return p.publish(ctx, "email.verification_requested", event)
}

func (p *httpPublisher) PublishPasswordResetRequested(ctx context.Context, event PasswordResetRequestedEvent) error {
	return p.publish(ctx, "password.reset_requested", event)
}
