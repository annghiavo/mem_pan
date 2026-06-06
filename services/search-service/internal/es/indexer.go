package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DeckDoc struct {
	DeckID      string    `json:"deck_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	Status      string    `json:"status"`
	CardCount   int32     `json:"card_count"`
	ClonedFrom  string    `json:"cloned_from,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FolderDoc struct {
	FolderID    string    `json:"folder_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CardDoc struct {
	CardID       string    `json:"card_id"`
	UserID       string    `json:"user_id"`
	DeckID       string    `json:"deck_id"`
	NoteID       string    `json:"note_id"`
	ContentFront string    `json:"content_front"`
	ContentBack  string    `json:"content_back"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserDoc struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) IndexDeck(ctx context.Context, d DeckDoc) error {
	return c.indexDoc(ctx, c.Indices.Deck, d.DeckID, d)
}

func (c *Client) IndexFolder(ctx context.Context, d FolderDoc) error {
	return c.indexDoc(ctx, c.Indices.Folder, d.FolderID, d)
}

func (c *Client) IndexCard(ctx context.Context, d CardDoc) error {
	return c.indexDoc(ctx, c.Indices.Card, d.CardID, d)
}

func (c *Client) IndexUser(ctx context.Context, d UserDoc) error {
	return c.indexDoc(ctx, c.Indices.User, d.UserID, d)
}

func (c *Client) DeleteDeck(ctx context.Context, id string) error {
	return c.deleteDoc(ctx, c.Indices.Deck, id)
}

func (c *Client) DeleteFolder(ctx context.Context, id string) error {
	return c.deleteDoc(ctx, c.Indices.Folder, id)
}

func (c *Client) DeleteCard(ctx context.Context, id string) error {
	return c.deleteDoc(ctx, c.Indices.Card, id)
}

func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.deleteDoc(ctx, c.Indices.User, id)
}

// DeleteCardsByDeckID removes ALL card documents that belong to deckID using
// Elasticsearch delete-by-query. This must be called whenever a deck is deleted
// so that orphaned card documents cannot surface in Phase-2 card-content search.
func (c *Client) DeleteCardsByDeckID(ctx context.Context, deckID string) error {
	body, err := json.Marshal(map[string]any{
		"query": map[string]any{
			"term": map[string]any{"deck_id": deckID},
		},
	})
	if err != nil {
		return err
	}
	res, err := c.ES.DeleteByQuery(
		[]string{c.Indices.Card},
		bytes.NewReader(body),
		c.ES.DeleteByQuery.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("delete cards by deck %s: %s", deckID, res.String())
	}
	return nil
}

// UpdateDeckPartial applies a partial JSON-merge update to a deck document.
// If the document does not exist, it is upserted with the merged fields.
func (c *Client) UpdateDeckPartial(ctx context.Context, id string, partial map[string]any) error {
	return c.updateDoc(ctx, c.Indices.Deck, id, partial)
}

func (c *Client) UpdateFolderPartial(ctx context.Context, id string, partial map[string]any) error {
	return c.updateDoc(ctx, c.Indices.Folder, id, partial)
}

func (c *Client) UpdateCardPartial(ctx context.Context, id string, partial map[string]any) error {
	return c.updateDoc(ctx, c.Indices.Card, id, partial)
}

func (c *Client) UpdateUserPartial(ctx context.Context, id string, partial map[string]any) error {
	return c.updateDoc(ctx, c.Indices.User, id, partial)
}

func (c *Client) indexDoc(ctx context.Context, index, id string, doc any) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	res, err := c.ES.Index(
		index,
		bytes.NewReader(body),
		c.ES.Index.WithContext(ctx),
		c.ES.Index.WithDocumentID(id),
		c.ES.Index.WithRefresh("false"),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("index %s/%s: %s", index, id, res.String())
	}
	return nil
}

func (c *Client) deleteDoc(ctx context.Context, index, id string) error {
	res, err := c.ES.Delete(index, id, c.ES.Delete.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete %s/%s: %s", index, id, res.String())
	}
	return nil
}

func (c *Client) updateDoc(ctx context.Context, index, id string, partial map[string]any) error {
	body, err := json.Marshal(map[string]any{
		"doc":           partial,
		"doc_as_upsert": true,
	})
	if err != nil {
		return err
	}
	res, err := c.ES.Update(
		index, id,
		strings.NewReader(string(body)),
		c.ES.Update.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("update %s/%s: %s", index, id, res.String())
	}
	return nil
}

// BumpDeckCardCount atomically adjusts the deck's card_count by delta using
// a painless script. Used so card.created / card.deleted events keep search
// results' card count in sync without racing on read-modify-write.
func (c *Client) BumpDeckCardCount(ctx context.Context, deckID string, delta int) error {
	body, err := json.Marshal(map[string]any{
		"script": map[string]any{
			"source": "ctx._source.card_count = (ctx._source.card_count == null ? 0 : ctx._source.card_count) + params.delta; if (ctx._source.card_count < 0) { ctx._source.card_count = 0 }",
			"lang":   "painless",
			"params": map[string]any{"delta": delta},
		},
	})
	if err != nil {
		return err
	}
	res, err := c.ES.Update(
		c.Indices.Deck, deckID,
		strings.NewReader(string(body)),
		c.ES.Update.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// 404 means the deck document was not yet indexed (e.g. event race) —
	// safe to ignore, the deck.created/updated event will set the count.
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("bump card_count %s: %s", deckID, res.String())
	}
	return nil
}
