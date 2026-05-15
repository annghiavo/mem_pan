package subscriber

import (
	"context"
	"encoding/json"
	"log"

	"mem_pan/services/search-service/internal/es"
	"mem_pan/services/search-service/internal/events"
	"mem_pan/services/search-service/internal/service"
)

type Handler struct {
	svc service.SearchService
}

func NewHandler(svc service.SearchService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Dispatch(ctx context.Context, eventType string, data []byte) error {
	switch eventType {
	case events.TypeUserRegistered:
		return h.handle(ctx, data, h.userRegistered)
	case events.TypeUserUpdated:
		return h.handle(ctx, data, h.userUpdated)
	case events.TypeUserDeleted:
		return h.handle(ctx, data, h.userDeleted)

	case events.TypeDeckCreated:
		return h.handle(ctx, data, h.deckCreated)
	case events.TypeDeckUpdated:
		return h.handle(ctx, data, h.deckUpdated)
	case events.TypeDeckDeleted:
		return h.handle(ctx, data, h.deckDeleted)

	case events.TypeFolderCreated:
		return h.handle(ctx, data, h.folderCreated)
	case events.TypeFolderUpdated:
		return h.handle(ctx, data, h.folderUpdated)
	case events.TypeFolderDeleted:
		return h.handle(ctx, data, h.folderDeleted)

	case events.TypeCardCreated:
		return h.handle(ctx, data, h.cardCreated)
	case events.TypeCardUpdated:
		return h.handle(ctx, data, h.cardUpdated)
	case events.TypeCardDeleted:
		return h.handle(ctx, data, h.cardDeleted)

	default:
		log.Printf("[search] unknown event type %q — skipping", eventType)
		return nil
	}
}

func (h *Handler) handle(ctx context.Context, data []byte, fn func(ctx context.Context, data []byte) error) error {
	return fn(ctx, data)
}

func (h *Handler) userRegistered(ctx context.Context, data []byte) error {
	var e events.UserRegistered
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.IndexUser(ctx, es.UserDoc{
		UserID:    e.UserID,
		Username:  e.Username,
		FullName:  e.FullName,
		AvatarURL: e.AvatarURL,
		CreatedAt: e.CreatedAt,
	})
}

func (h *Handler) userUpdated(ctx context.Context, data []byte) error {
	var e events.UserUpdated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	partial := map[string]any{}
	if e.Username != "" {
		partial["username"] = e.Username
	}
	if e.FullName != "" {
		partial["full_name"] = e.FullName
	}
	if e.AvatarURL != "" {
		partial["avatar_url"] = e.AvatarURL
	}
	if len(partial) == 0 {
		return nil
	}
	return h.svc.UpdateUser(ctx, e.UserID, partial)
}

func (h *Handler) userDeleted(ctx context.Context, data []byte) error {
	var e events.UserDeleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.DeleteUser(ctx, e.UserID)
}

func (h *Handler) deckCreated(ctx context.Context, data []byte) error {
	var e events.DeckCreated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.IndexDeck(ctx, es.DeckDoc{
		DeckID:      e.DeckID,
		UserID:      e.UserID,
		Name:        e.DeckName,
		Description: e.Description,
		IsPublic:    e.IsPublic,
		Status:      "active",
		CardCount:   e.CardCount,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.CreatedAt,
	})
}

func (h *Handler) deckUpdated(ctx context.Context, data []byte) error {
	var e events.DeckUpdated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	partial := map[string]any{
		"name":        e.DeckName,
		"description": e.Description,
		"is_public":   e.IsPublic,
		"card_count":  e.CardCount,
		"user_id":     e.UserID,
		"updated_at":  e.UpdatedAt,
	}
	return h.svc.UpdateDeck(ctx, e.DeckID, partial)
}

func (h *Handler) deckDeleted(ctx context.Context, data []byte) error {
	var e events.DeckDeleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.DeleteDeck(ctx, e.DeckID)
}

func (h *Handler) folderCreated(ctx context.Context, data []byte) error {
	var e events.FolderCreated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.IndexFolder(ctx, es.FolderDoc{
		FolderID:    e.FolderID,
		UserID:      e.UserID,
		Name:        e.Name,
		Description: e.Description,
		IsPublic:    e.IsPublic,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.CreatedAt,
	})
}

func (h *Handler) folderUpdated(ctx context.Context, data []byte) error {
	var e events.FolderUpdated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	partial := map[string]any{
		"name":        e.Name,
		"description": e.Description,
		"is_public":   e.IsPublic,
		"user_id":     e.UserID,
		"updated_at":  e.UpdatedAt,
	}
	return h.svc.UpdateFolder(ctx, e.FolderID, partial)
}

func (h *Handler) folderDeleted(ctx context.Context, data []byte) error {
	var e events.FolderDeleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.DeleteFolder(ctx, e.FolderID)
}

func (h *Handler) cardCreated(ctx context.Context, data []byte) error {
	var e events.CardCreated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.IndexCard(ctx, es.CardDoc{
		CardID:       e.CardID,
		UserID:       e.UserID,
		DeckID:       e.DeckID,
		NoteID:       e.NoteID,
		ContentFront: e.ContentFront,
		ContentBack:  e.ContentBack,
		CreatedAt:    e.CreatedAt,
	})
}

func (h *Handler) cardUpdated(ctx context.Context, data []byte) error {
	var e events.CardUpdated
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	partial := map[string]any{
		"deck_id":       e.DeckID,
		"user_id":       e.UserID,
		"note_id":       e.NoteID,
		"content_front": e.ContentFront,
		"content_back":  e.ContentBack,
	}
	return h.svc.UpdateCard(ctx, e.CardID, partial)
}

func (h *Handler) cardDeleted(ctx context.Context, data []byte) error {
	var e events.CardDeleted
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return h.svc.DeleteCard(ctx, e.CardID)
}
