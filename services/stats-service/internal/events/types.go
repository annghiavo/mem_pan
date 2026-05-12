package events

import "time"

const (
	TypeUserRegistered = "user.registered"
	TypeDeckCreated    = "deck.created"
	TypeDeckUpdated    = "deck.updated"
	TypeCardCreated    = "card.created"
	TypeCardReviewed   = "card.reviewed"
)

type Envelope struct {
	EventType string `json:"event_type"`
	Data      []byte `json:"data"`
}

// Published by auth-service on user.registered
type UserRegistered struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

// Published by deck-service on deck.created
type DeckCreated struct {
	DeckID    string    `json:"deck_id"`
	UserID    string    `json:"user_id"`
	DeckName  string    `json:"deck_name"`
	CreatedAt time.Time `json:"created_at"`
}

// Published by deck-service on deck.updated
type DeckUpdated struct {
	DeckID   string `json:"deck_id"`
	UserID   string `json:"user_id"`
	DeckName string `json:"deck_name"`
}

// Published by deck-service on card.created
type CardCreated struct {
	CardID    string    `json:"card_id"`
	DeckID    string    `json:"deck_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Published by study-service on card.reviewed
type CardReviewed struct {
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
