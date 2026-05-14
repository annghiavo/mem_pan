package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type Hit struct {
	ID     string          `json:"_id"`
	Index  string          `json:"_index"`
	Score  float64         `json:"_score"`
	Source json.RawMessage `json:"_source"`
}

type SearchResult struct {
	Total int64
	Hits  []Hit
}

type searchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []Hit `json:"hits"`
	} `json:"hits"`
}

// DoSearch runs a raw search body against the given index. Used by typed search builders below.
func (c *Client) DoSearch(ctx context.Context, index string, body map[string]any) (*SearchResult, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	res, err := c.ES.Search(
		c.ES.Search.WithContext(ctx),
		c.ES.Search.WithIndex(index),
		c.ES.Search.WithBody(bytes.NewReader(buf)),
		c.ES.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("search %s: %s", index, res.String())
	}
	var sr searchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	return &SearchResult{Total: sr.Hits.Total.Value, Hits: sr.Hits.Hits}, nil
}

func (sr *SearchResult) Decode(i int, out any) error {
	return json.Unmarshal(sr.Hits[i].Source, out)
}

// MultiMatchOrMatchAll returns a query clause that matches all docs when q is empty,
// otherwise a multi_match across the given fields.
func MultiMatchOrMatchAll(q string, fields []string) map[string]any {
	if q == "" {
		return map[string]any{"match_all": map[string]any{}}
	}
	return map[string]any{
		"multi_match": map[string]any{
			"query":  q,
			"fields": fields,
			"type":   "best_fields",
		},
	}
}

// BoolQuery composes must/filter/should clauses; nil slices are omitted.
func BoolQuery(must, filter []map[string]any) map[string]any {
	clauses := map[string]any{}
	if len(must) > 0 {
		clauses["must"] = must
	}
	if len(filter) > 0 {
		clauses["filter"] = filter
	}
	return map[string]any{"bool": clauses}
}

func PageOffset(page, size int32) (from, sz int) {
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if page <= 0 {
		page = 1
	}
	return int((page - 1) * size), int(size)
}
