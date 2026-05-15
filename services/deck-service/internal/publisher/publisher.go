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
	DeckID      string    `json:"deck_id"`
	UserID      string    `json:"user_id"`
	DeckName    string    `json:"deck_name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CardCount   int32     `json:"card_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type DeckUpdatedEvent struct {
	DeckID      string    `json:"deck_id"`
	UserID      string    `json:"user_id"`
	DeckName    string    `json:"deck_name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CardCount   int32     `json:"card_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DeckDeletedEvent struct {
	DeckID string `json:"deck_id"`
	UserID string `json:"user_id"`
}

type FolderCreatedEvent struct {
	FolderID    string    `json:"folder_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

type FolderUpdatedEvent struct {
	FolderID    string    `json:"folder_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FolderDeletedEvent struct {
	FolderID string `json:"folder_id"`
	UserID   string `json:"user_id"`
}

type CardCreatedEvent struct {
	CardID       string    `json:"card_id"`
	DeckID       string    `json:"deck_id"`
	UserID       string    `json:"user_id"`
	NoteID       string    `json:"note_id"`
	ContentFront string    `json:"content_front"`
	ContentBack  string    `json:"content_back"`
	CreatedAt    time.Time `json:"created_at"`
}

type CardUpdatedEvent struct {
	CardID       string `json:"card_id"`
	DeckID       string `json:"deck_id"`
	UserID       string `json:"user_id"`
	NoteID       string `json:"note_id"`
	ContentFront string `json:"content_front"`
	ContentBack  string `json:"content_back"`
}

type CardDeletedEvent struct {
	CardID string `json:"card_id"`
	DeckID string `json:"deck_id"`
	UserID string `json:"user_id"`
}

// ReportSubmittedEvent is fired when a user files a moderation report.
// admin-service consumes this and persists it to admin_db.reports.
type ReportSubmittedEvent struct {
	ReporterID     string    `json:"reporter_id"`
	TargetType     string    `json:"target_type"`     // deck | user | note
	TargetID       string    `json:"target_id"`
	ReasonCategory string    `json:"reason_category"` // inappropriate_content | copyright_violation | spam | harassment | misinformation | other
	Description    string    `json:"description"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

type EventPublisher interface {
	PublishDeckCreated(ctx context.Context, event DeckCreatedEvent) error
	PublishDeckUpdated(ctx context.Context, event DeckUpdatedEvent) error
	PublishDeckDeleted(ctx context.Context, event DeckDeletedEvent) error
	PublishFolderCreated(ctx context.Context, event FolderCreatedEvent) error
	PublishFolderUpdated(ctx context.Context, event FolderUpdatedEvent) error
	PublishFolderDeleted(ctx context.Context, event FolderDeletedEvent) error
	PublishCardCreated(ctx context.Context, event CardCreatedEvent) error
	PublishCardUpdated(ctx context.Context, event CardUpdatedEvent) error
	PublishCardDeleted(ctx context.Context, event CardDeletedEvent) error
	PublishReportSubmitted(ctx context.Context, event ReportSubmittedEvent) error
}

// noopPublisher logs events without sending them anywhere.
type noopPublisher struct{}

func NewNoopPublisher() EventPublisher { return &noopPublisher{} }

func (p *noopPublisher) logEvent(kind string, event any) error {
	b, _ := json.Marshal(event)
	log.Printf("[event] %s: %s", kind, b)
	return nil
}

func (p *noopPublisher) PublishDeckCreated(_ context.Context, e DeckCreatedEvent) error {
	return p.logEvent("deck.created", e)
}
func (p *noopPublisher) PublishDeckUpdated(_ context.Context, e DeckUpdatedEvent) error {
	return p.logEvent("deck.updated", e)
}
func (p *noopPublisher) PublishDeckDeleted(_ context.Context, e DeckDeletedEvent) error {
	return p.logEvent("deck.deleted", e)
}
func (p *noopPublisher) PublishFolderCreated(_ context.Context, e FolderCreatedEvent) error {
	return p.logEvent("folder.created", e)
}
func (p *noopPublisher) PublishFolderUpdated(_ context.Context, e FolderUpdatedEvent) error {
	return p.logEvent("folder.updated", e)
}
func (p *noopPublisher) PublishFolderDeleted(_ context.Context, e FolderDeletedEvent) error {
	return p.logEvent("folder.deleted", e)
}
func (p *noopPublisher) PublishCardCreated(_ context.Context, e CardCreatedEvent) error {
	return p.logEvent("card.created", e)
}
func (p *noopPublisher) PublishCardUpdated(_ context.Context, e CardUpdatedEvent) error {
	return p.logEvent("card.updated", e)
}
func (p *noopPublisher) PublishCardDeleted(_ context.Context, e CardDeletedEvent) error {
	return p.logEvent("card.deleted", e)
}
func (p *noopPublisher) PublishReportSubmitted(_ context.Context, e ReportSubmittedEvent) error {
	return p.logEvent("report.submitted", e)
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

func (p *httpPublisher) PublishDeckCreated(ctx context.Context, e DeckCreatedEvent) error {
	return p.publish(ctx, "deck.created", e)
}
func (p *httpPublisher) PublishDeckUpdated(ctx context.Context, e DeckUpdatedEvent) error {
	return p.publish(ctx, "deck.updated", e)
}
func (p *httpPublisher) PublishDeckDeleted(ctx context.Context, e DeckDeletedEvent) error {
	return p.publish(ctx, "deck.deleted", e)
}
func (p *httpPublisher) PublishFolderCreated(ctx context.Context, e FolderCreatedEvent) error {
	return p.publish(ctx, "folder.created", e)
}
func (p *httpPublisher) PublishFolderUpdated(ctx context.Context, e FolderUpdatedEvent) error {
	return p.publish(ctx, "folder.updated", e)
}
func (p *httpPublisher) PublishFolderDeleted(ctx context.Context, e FolderDeletedEvent) error {
	return p.publish(ctx, "folder.deleted", e)
}
func (p *httpPublisher) PublishCardCreated(ctx context.Context, e CardCreatedEvent) error {
	return p.publish(ctx, "card.created", e)
}
func (p *httpPublisher) PublishCardUpdated(ctx context.Context, e CardUpdatedEvent) error {
	return p.publish(ctx, "card.updated", e)
}
func (p *httpPublisher) PublishCardDeleted(ctx context.Context, e CardDeletedEvent) error {
	return p.publish(ctx, "card.deleted", e)
}
func (p *httpPublisher) PublishReportSubmitted(ctx context.Context, e ReportSubmittedEvent) error {
	return p.publish(ctx, "report.submitted", e)
}
