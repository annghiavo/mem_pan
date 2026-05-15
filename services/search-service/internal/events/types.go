package events

import "time"

// Event type constants — must match the strings published by other services.
const (
	// auth-service (user-events topic)
	TypeUserRegistered = "user.registered"
	TypeUserUpdated    = "user.updated"
	TypeUserDeleted    = "user.deleted"

	// deck-service (deck-events topic)
	TypeDeckCreated   = "deck.created"
	TypeDeckUpdated   = "deck.updated"
	TypeDeckDeleted   = "deck.deleted"
	TypeFolderCreated = "folder.created"
	TypeFolderUpdated = "folder.updated"
	TypeFolderDeleted = "folder.deleted"
	TypeCardCreated   = "card.created"
	TypeCardUpdated   = "card.updated"
	TypeCardDeleted   = "card.deleted"
)

type Envelope struct {
	EventType string `json:"event_type"`
	Data      []byte `json:"data"`
}

type UserRegistered struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

type UserUpdated struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
}

type UserDeleted struct {
	UserID string `json:"user_id"`
}

type DeckCreated struct {
	DeckID      string    `json:"deck_id"`
	UserID      string    `json:"user_id"`
	DeckName    string    `json:"deck_name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CardCount   int32     `json:"card_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type DeckUpdated struct {
	DeckID      string    `json:"deck_id"`
	UserID      string    `json:"user_id"`
	DeckName    string    `json:"deck_name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CardCount   int32     `json:"card_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DeckDeleted struct {
	DeckID string `json:"deck_id"`
	UserID string `json:"user_id"`
}

type FolderCreated struct {
	FolderID    string    `json:"folder_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

type FolderUpdated struct {
	FolderID    string    `json:"folder_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FolderDeleted struct {
	FolderID string `json:"folder_id"`
	UserID   string `json:"user_id"`
}

type CardCreated struct {
	CardID       string    `json:"card_id"`
	DeckID       string    `json:"deck_id"`
	UserID       string    `json:"user_id"`
	NoteID       string    `json:"note_id"`
	ContentFront string    `json:"content_front"`
	ContentBack  string    `json:"content_back"`
	CreatedAt    time.Time `json:"created_at"`
}

type CardUpdated struct {
	CardID       string `json:"card_id"`
	DeckID       string `json:"deck_id"`
	UserID       string `json:"user_id"`
	NoteID       string `json:"note_id"`
	ContentFront string `json:"content_front"`
	ContentBack  string `json:"content_back"`
}

type CardDeleted struct {
	CardID string `json:"card_id"`
	UserID string `json:"user_id"`
	DeckID string `json:"deck_id"`
}
