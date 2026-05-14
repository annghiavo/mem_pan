package service

import (
	"context"

	"mem_pan/services/search-service/internal/es"
)

type SearchService interface {
	// Indexing
	IndexDeck(ctx context.Context, d es.DeckDoc) error
	UpdateDeck(ctx context.Context, id string, partial map[string]any) error
	DeleteDeck(ctx context.Context, id string) error
	IndexFolder(ctx context.Context, d es.FolderDoc) error
	UpdateFolder(ctx context.Context, id string, partial map[string]any) error
	DeleteFolder(ctx context.Context, id string) error
	IndexCard(ctx context.Context, d es.CardDoc) error
	UpdateCard(ctx context.Context, id string, partial map[string]any) error
	DeleteCard(ctx context.Context, id string) error
	IndexUser(ctx context.Context, d es.UserDoc) error
	UpdateUser(ctx context.Context, id string, partial map[string]any) error
	DeleteUser(ctx context.Context, id string) error

	// Searching
	SearchDecks(ctx context.Context, p DeckSearchParams) (*es.SearchResult, error)
	SearchFolders(ctx context.Context, p FolderSearchParams) (*es.SearchResult, error)
	SearchCards(ctx context.Context, p CardSearchParams) (*es.SearchResult, error)
	SearchUsers(ctx context.Context, p UserSearchParams) (*es.SearchResult, error)
}

type DeckScope int

const (
	DeckScopePublic DeckScope = iota
	DeckScopeMine
	DeckScopeAll
)

type DeckSearchParams struct {
	Query    string
	Scope    DeckScope
	CallerID string // empty when caller is unauthenticated
	Page     int32
	PageSize int32
}

type FolderSearchParams struct {
	Query    string
	UserID   string // required
	Page     int32
	PageSize int32
}

type CardSearchParams struct {
	Query    string
	UserID   string // required
	DeckID   string // optional
	Page     int32
	PageSize int32
}

type UserSearchParams struct {
	Query    string
	Page     int32
	PageSize int32
}

type service struct {
	client *es.Client
}

func New(client *es.Client) SearchService {
	return &service{client: client}
}

// ---- indexing pass-through ----

func (s *service) IndexDeck(ctx context.Context, d es.DeckDoc) error { return s.client.IndexDeck(ctx, d) }
func (s *service) UpdateDeck(ctx context.Context, id string, p map[string]any) error {
	return s.client.UpdateDeckPartial(ctx, id, p)
}
func (s *service) DeleteDeck(ctx context.Context, id string) error { return s.client.DeleteDeck(ctx, id) }

func (s *service) IndexFolder(ctx context.Context, d es.FolderDoc) error {
	return s.client.IndexFolder(ctx, d)
}
func (s *service) UpdateFolder(ctx context.Context, id string, p map[string]any) error {
	return s.client.UpdateFolderPartial(ctx, id, p)
}
func (s *service) DeleteFolder(ctx context.Context, id string) error {
	return s.client.DeleteFolder(ctx, id)
}

func (s *service) IndexCard(ctx context.Context, d es.CardDoc) error { return s.client.IndexCard(ctx, d) }
func (s *service) UpdateCard(ctx context.Context, id string, p map[string]any) error {
	return s.client.UpdateCardPartial(ctx, id, p)
}
func (s *service) DeleteCard(ctx context.Context, id string) error { return s.client.DeleteCard(ctx, id) }

func (s *service) IndexUser(ctx context.Context, d es.UserDoc) error { return s.client.IndexUser(ctx, d) }
func (s *service) UpdateUser(ctx context.Context, id string, p map[string]any) error {
	return s.client.UpdateUserPartial(ctx, id, p)
}
func (s *service) DeleteUser(ctx context.Context, id string) error { return s.client.DeleteUser(ctx, id) }

// ---- searching ----

func (s *service) SearchDecks(ctx context.Context, p DeckSearchParams) (*es.SearchResult, error) {
	from, size := es.PageOffset(p.Page, p.PageSize)
	must := []map[string]any{es.MultiMatchOrMatchAll(p.Query, []string{"name^3", "description"})}

	var filters []map[string]any
	filters = append(filters, map[string]any{"term": map[string]any{"status": "active"}})

	switch p.Scope {
	case DeckScopeMine:
		if p.CallerID == "" {
			return &es.SearchResult{Hits: []es.Hit{}}, nil
		}
		filters = append(filters, map[string]any{"term": map[string]any{"user_id": p.CallerID}})
	case DeckScopeAll:
		// Caller's own decks ∪ public decks.
		if p.CallerID != "" {
			filters = append(filters, map[string]any{
				"bool": map[string]any{
					"should": []map[string]any{
						{"term": map[string]any{"is_public": true}},
						{"term": map[string]any{"user_id": p.CallerID}},
					},
					"minimum_should_match": 1,
				},
			})
		} else {
			filters = append(filters, map[string]any{"term": map[string]any{"is_public": true}})
		}
	default: // DeckScopePublic
		filters = append(filters, map[string]any{"term": map[string]any{"is_public": true}})
	}

	body := map[string]any{
		"from":  from,
		"size":  size,
		"query": es.BoolQuery(must, filters),
	}
	return s.client.DoSearch(ctx, s.client.Indices.Deck, body)
}

func (s *service) SearchFolders(ctx context.Context, p FolderSearchParams) (*es.SearchResult, error) {
	from, size := es.PageOffset(p.Page, p.PageSize)
	must := []map[string]any{es.MultiMatchOrMatchAll(p.Query, []string{"name^3", "description"})}
	filters := []map[string]any{{"term": map[string]any{"user_id": p.UserID}}}
	body := map[string]any{
		"from":  from,
		"size":  size,
		"query": es.BoolQuery(must, filters),
	}
	return s.client.DoSearch(ctx, s.client.Indices.Folder, body)
}

func (s *service) SearchCards(ctx context.Context, p CardSearchParams) (*es.SearchResult, error) {
	from, size := es.PageOffset(p.Page, p.PageSize)
	must := []map[string]any{es.MultiMatchOrMatchAll(p.Query, []string{"content_front^2", "content_back"})}
	filters := []map[string]any{{"term": map[string]any{"user_id": p.UserID}}}
	if p.DeckID != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"deck_id": p.DeckID}})
	}
	body := map[string]any{
		"from":  from,
		"size":  size,
		"query": es.BoolQuery(must, filters),
	}
	return s.client.DoSearch(ctx, s.client.Indices.Card, body)
}

func (s *service) SearchUsers(ctx context.Context, p UserSearchParams) (*es.SearchResult, error) {
	from, size := es.PageOffset(p.Page, p.PageSize)
	must := []map[string]any{es.MultiMatchOrMatchAll(p.Query, []string{"username^3", "full_name"})}
	body := map[string]any{
		"from":  from,
		"size":  size,
		"query": es.BoolQuery(must, nil),
	}
	return s.client.DoSearch(ctx, s.client.Indices.User, body)
}
