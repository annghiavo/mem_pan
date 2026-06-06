package service

import (
	"context"
	"encoding/json"
	"log"

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
	BumpDeckCardCount(ctx context.Context, deckID string, delta int) error
	// DeleteCardsByDeck purges ALL card documents for a deck from the search index.
	// Must be called when a deck is deleted to prevent orphaned card docs from
	// appearing in card-content search results.
	DeleteCardsByDeck(ctx context.Context, deckID string) error
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

type FolderScope int

const (
	FolderScopePublic FolderScope = iota
	FolderScopeMine
	FolderScopeAll
)

type FolderSearchParams struct {
	Query    string
	Scope    FolderScope
	CallerID string // empty when caller is unauthenticated
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
func (s *service) BumpDeckCardCount(ctx context.Context, deckID string, delta int) error {
	return s.client.BumpDeckCardCount(ctx, deckID, delta)
}
func (s *service) DeleteCardsByDeck(ctx context.Context, deckID string) error {
	return s.client.DeleteCardsByDeckID(ctx, deckID)
}

func (s *service) IndexUser(ctx context.Context, d es.UserDoc) error { return s.client.IndexUser(ctx, d) }
func (s *service) UpdateUser(ctx context.Context, id string, p map[string]any) error {
	return s.client.UpdateUserPartial(ctx, id, p)
}
func (s *service) DeleteUser(ctx context.Context, id string) error { return s.client.DeleteUser(ctx, id) }

// ---- searching ----

func (s *service) SearchDecks(ctx context.Context, p DeckSearchParams) (*es.SearchResult, error) {
	from, size := es.PageOffset(p.Page, p.PageSize)

	// Build visibility / scope filters (shared between both search phases).
	scopeFilters, early := buildDeckScopeFilters(p)
	if early != nil {
		return early, nil
	}

	// Phase 1: search decks by name and description.
	must := []map[string]any{es.MultiMatchOrMatchAll(p.Query, []string{"name^3", "description"})}
	body := map[string]any{
		"from":  from,
		"size":  size,
		"query": es.BoolQuery(must, scopeFilters),
	}
	result, err := s.client.DoSearch(ctx, s.client.Indices.Deck, body)
	if err != nil {
		return nil, err
	}

	// Phase 2 (only when user typed something): search card content and surface
	// parent decks that weren't already returned by phase 1.
	if p.Query != "" {
		s.enrichWithCardContent(ctx, result, p.Query, p.CallerID, scopeFilters, size)
	}

	return result, nil
}

// buildDeckScopeFilters returns the Elasticsearch filter clauses that enforce
// deck visibility for the given scope. Returns a non-nil early result when the
// query should immediately return empty (e.g. MINE scope without auth).
func buildDeckScopeFilters(p DeckSearchParams) (filters []map[string]any, early *es.SearchResult) {
	filters = append(filters, map[string]any{"term": map[string]any{"status": "active"}})
	switch p.Scope {
	case DeckScopeMine:
		if p.CallerID == "" {
			return nil, &es.SearchResult{Hits: []es.Hit{}}
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
	return filters, nil
}

// enrichWithCardContent performs a second ES query against the cards index,
// collects the parent deck_ids of matching cards, then fetches those decks
// (applying the same scope filters) and appends any new ones to result.
// Errors are logged and silently ignored so the primary result is always returned.
func (s *service) enrichWithCardContent(
	ctx context.Context,
	result *es.SearchResult,
	query, callerID string,
	scopeFilters []map[string]any,
	maxExtra int,
) {
	// Track deck IDs already present so we don't return duplicates.
	seen := make(map[string]bool, len(result.Hits))
	for _, h := range result.Hits {
		seen[h.ID] = true
	}

	// Search cards: if the caller is known, limit to their own cards only
	// (they can't see other users' private card content).
	var cardFilters []map[string]any
	if callerID != "" {
		cardFilters = append(cardFilters, map[string]any{"term": map[string]any{"user_id": callerID}})
	}
	cardBody := map[string]any{
		"size":    50, // fetch up to 50 card hits to find distinct deck IDs
		"_source": []string{"deck_id"},
		"query": es.BoolQuery(
			[]map[string]any{es.MultiMatchOrMatchAll(query, []string{"content_front^2", "content_back"})},
			cardFilters,
		),
	}
	cardResult, err := s.client.DoSearch(ctx, s.client.Indices.Card, cardBody)
	if err != nil {
		log.Printf("[search] card content phase error: %v", err)
		return
	}

	// Collect unique deck IDs that aren't already in the primary result.
	var newDeckIDs []any
	for _, h := range cardResult.Hits {
		var c es.CardDoc
		if err := json.Unmarshal(h.Source, &c); err != nil || c.DeckID == "" {
			continue
		}
		if seen[c.DeckID] {
			continue
		}
		seen[c.DeckID] = true
		newDeckIDs = append(newDeckIDs, c.DeckID)
	}
	if len(newDeckIDs) == 0 {
		return
	}

	// Fetch those decks, applying the same scope/visibility filters so we never
	// leak private decks. We use ES "ids" query (matches on _id field) which
	// is very cheap — no scoring overhead.
	extraFilters := append(append([]map[string]any{}, scopeFilters...),
		map[string]any{"ids": map[string]any{"values": newDeckIDs}},
	)
	extraSize := len(newDeckIDs)
	if extraSize > maxExtra {
		extraSize = maxExtra
	}
	extraBody := map[string]any{
		"size":  extraSize,
		"query": es.BoolQuery(nil, extraFilters),
	}
	extraResult, err := s.client.DoSearch(ctx, s.client.Indices.Deck, extraBody)
	if err != nil {
		log.Printf("[search] card-sourced deck fetch error: %v", err)
		return
	}

	result.Hits = append(result.Hits, extraResult.Hits...)
	result.Total += extraResult.Total
}

func (s *service) SearchFolders(ctx context.Context, p FolderSearchParams) (*es.SearchResult, error) {
	from, size := es.PageOffset(p.Page, p.PageSize)
	must := []map[string]any{es.MultiMatchOrMatchAll(p.Query, []string{"name^3", "description"})}

	var filters []map[string]any
	switch p.Scope {
	case FolderScopeMine:
		if p.CallerID == "" {
			return &es.SearchResult{Hits: []es.Hit{}}, nil
		}
		filters = append(filters, map[string]any{"term": map[string]any{"user_id": p.CallerID}})
	case FolderScopeAll:
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
	default: // FolderScopePublic
		filters = append(filters, map[string]any{"term": map[string]any{"is_public": true}})
	}

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
