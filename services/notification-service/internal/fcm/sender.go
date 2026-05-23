package fcm

import (
	"context"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// tokenPrefix returns a short, non-sensitive prefix of an FCM token for log
// correlation without leaking the whole secret.
func tokenPrefix(t string) string {
	if len(t) <= 12 {
		return t
	}
	return t[:12] + "…"
}

// Sender sends Firebase Cloud Messaging push notifications.
type Sender interface {
	Send(ctx context.Context, tokens []string, title, body string, data map[string]string) error
}

type firebaseSender struct {
	client *messaging.Client
}

// New creates a Sender backed by the Firebase Admin SDK.
// credentialsFile is the path to a service account JSON key file.
// If empty, Application Default Credentials are used.
func New(ctx context.Context, projectID, credentialsFile string) (Sender, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, opts...)
	if err != nil {
		return nil, fmt.Errorf("fcm: init firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("fcm: init messaging client: %w", err)
	}

	return &firebaseSender{client: client}, nil
}

// Send delivers a notification to all provided device tokens using multicast.
// Tokens that fail silently (expired, unregistered) are logged but do not
// cause the call to return an error.
func (s *firebaseSender) Send(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	if len(tokens) == 0 {
		return nil
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	resp, err := s.client.SendEachForMulticast(ctx, msg)
	if err != nil {
		return fmt.Errorf("fcm: multicast send: %w", err)
	}

	// Per-token failures (expired token, invalid registration, sender
	// mismatch, etc.) don't fail the multicast — FCM returns 200 with a
	// per-entry success flag. We log each failure so a misconfigured token
	// doesn't masquerade as a successful send.
	if resp.FailureCount > 0 {
		for i, r := range resp.Responses {
			if !r.Success {
				log.Printf("[fcm] token %s failed: %v",
					tokenPrefix(tokens[i]), r.Error)
			}
		}
	}
	if resp.FailureCount > 0 && resp.SuccessCount == 0 {
		return fmt.Errorf("fcm: all %d token(s) failed", resp.FailureCount)
	}
	return nil
}

// NoopSender is used when FCM is not configured.
type noopSender struct{}

func NewNoop() Sender { return &noopSender{} }

func (s *noopSender) Send(_ context.Context, tokens []string, title, body string, _ map[string]string) error {
	_ = tokens
	_ = title
	_ = body
	return nil
}
