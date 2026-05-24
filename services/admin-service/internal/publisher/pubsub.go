package publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"golang.org/x/oauth2/google"
	"log"
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
	if host != "" {
		return &httpPublisher{
			endpoint: fmt.Sprintf("http://%s/v1/projects/%s/topics/%s:publish", host, projectID, topicID),
			client:   &http.Client{},
		}
	}
	// Production: use metadata-server OAuth token (works on Cloud Run/GKE).
	client, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/pubsub")
	if err != nil {
		log.Printf("[publisher] failed to acquire pubsub credentials, falling back to noop: %v", err)
		return NewNoopPublisher()
	}
	return &httpPublisher{
		endpoint: fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/topics/%s:publish", projectID, topicID),
		client:   client,
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

func (p *httpPublisher) PublishReportResolved(ctx context.Context, event ReportResolvedEvent) error {
	return p.publish(ctx, "report.resolved", event)
}
