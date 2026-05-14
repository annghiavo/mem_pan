package es

import (
	"context"
	"fmt"
	"strings"
)

// EnsureIndices creates the four indices with their mappings if they do not already exist.
// Safe to call repeatedly on startup.
func (c *Client) EnsureIndices(ctx context.Context) error {
	specs := []struct {
		name    string
		mapping string
	}{
		{c.Indices.Deck, deckMapping},
		{c.Indices.Folder, folderMapping},
		{c.Indices.Card, cardMapping},
		{c.Indices.User, userMapping},
	}
	for _, s := range specs {
		exists, err := c.indexExists(ctx, s.name)
		if err != nil {
			return fmt.Errorf("check index %s: %w", s.name, err)
		}
		if exists {
			continue
		}
		if err := c.createIndex(ctx, s.name, s.mapping); err != nil {
			return fmt.Errorf("create index %s: %w", s.name, err)
		}
	}
	return nil
}

func (c *Client) indexExists(ctx context.Context, name string) (bool, error) {
	res, err := c.ES.Indices.Exists([]string{name}, c.ES.Indices.Exists.WithContext(ctx))
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		return true, nil
	}
	if res.StatusCode == 404 {
		return false, nil
	}
	return false, fmt.Errorf("unexpected status %s", res.Status())
}

func (c *Client) createIndex(ctx context.Context, name, mapping string) error {
	res, err := c.ES.Indices.Create(
		name,
		c.ES.Indices.Create.WithContext(ctx),
		c.ES.Indices.Create.WithBody(strings.NewReader(mapping)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("create %s: %s", name, res.String())
	}
	return nil
}

const deckMapping = `{
  "mappings": {
    "properties": {
      "deck_id":     { "type": "keyword" },
      "user_id":     { "type": "keyword" },
      "name":        { "type": "text", "fields": { "keyword": { "type": "keyword", "ignore_above": 256 } } },
      "description": { "type": "text" },
      "is_public":   { "type": "boolean" },
      "status":      { "type": "keyword" },
      "card_count":  { "type": "integer" },
      "cloned_from": { "type": "keyword" },
      "created_at":  { "type": "date" },
      "updated_at":  { "type": "date" }
    }
  }
}`

const folderMapping = `{
  "mappings": {
    "properties": {
      "folder_id":   { "type": "keyword" },
      "user_id":     { "type": "keyword" },
      "name":        { "type": "text", "fields": { "keyword": { "type": "keyword", "ignore_above": 256 } } },
      "description": { "type": "text" },
      "created_at":  { "type": "date" },
      "updated_at":  { "type": "date" }
    }
  }
}`

const cardMapping = `{
  "mappings": {
    "properties": {
      "card_id":       { "type": "keyword" },
      "user_id":       { "type": "keyword" },
      "deck_id":       { "type": "keyword" },
      "note_id":       { "type": "keyword" },
      "content_front": { "type": "text" },
      "content_back":  { "type": "text" },
      "created_at":    { "type": "date" }
    }
  }
}`

const userMapping = `{
  "mappings": {
    "properties": {
      "user_id":    { "type": "keyword" },
      "username":   { "type": "text", "fields": { "keyword": { "type": "keyword", "ignore_above": 256 } } },
      "full_name":  { "type": "text" },
      "avatar_url": { "type": "keyword" },
      "created_at": { "type": "date" }
    }
  }
}`
