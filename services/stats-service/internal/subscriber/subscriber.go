package subscriber

import (
	"context"
	"encoding/json"
	"log"

	"cloud.google.com/go/pubsub"

	"mem_pan/services/stats-service/internal/events"
)

type Subscriber struct {
	client  *pubsub.Client
	handler *Handler
}

func New(client *pubsub.Client, handler *Handler) *Subscriber {
	return &Subscriber{client: client, handler: handler}
}

// Start begins receiving messages on all three subscriptions concurrently.
// It blocks until ctx is cancelled.
func (s *Subscriber) Start(ctx context.Context, userSub, deckSub, studySub string) {
	go s.receive(ctx, userSub)
	go s.receive(ctx, deckSub)
	s.receive(ctx, studySub) // blocks in the main goroutine (last subscription)
}

func (s *Subscriber) receive(ctx context.Context, subID string) {
	sub := s.client.Subscription(subID)
	sub.ReceiveSettings.MaxOutstandingMessages = 10

	log.Printf("[pubsub] listening on subscription %s", subID)

	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		var env events.Envelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			log.Printf("[pubsub] malformed envelope on %s: %v — acking to discard", subID, err)
			msg.Ack()
			return
		}

		if err := s.handler.Dispatch(ctx, env.EventType, env.Data); err != nil {
			log.Printf("[pubsub] dispatch %s error: %v — nacking for retry", env.EventType, err)
			msg.Nack()
			return
		}

		msg.Ack()
	})

	if err != nil && ctx.Err() == nil {
		log.Printf("[pubsub] subscription %s receive error: %v", subID, err)
	}
}
