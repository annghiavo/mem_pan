package es

import (
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
)

type Indices struct {
	Deck   string
	Folder string
	Card   string
	User   string
}

type Client struct {
	ES      *elasticsearch.Client
	Indices Indices
}

func New(urls []string, apiKey string, indices Indices) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: urls,
		APIKey:    apiKey,
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch.NewClient: %w", err)
	}
	res, err := es.Info()
	if err != nil {
		return nil, fmt.Errorf("elasticsearch ping: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch ping returned %s", res.Status())
	}
	return &Client{ES: es, Indices: indices}, nil
}
