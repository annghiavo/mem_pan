//go:build integration

package service_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	tces "github.com/testcontainers/testcontainers-go/modules/elasticsearch"

	"mem_pan/services/search-service/internal/es"
	"mem_pan/services/search-service/internal/service"
)

var (
	testClient *es.Client
	testIdx    = es.Indices{
		Deck:   "decks_test",
		Folder: "folders_test",
		Card:   "cards_test",
		User:   "users_test",
	}
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	esContainer, err := tces.Run(ctx,
		"docker.elastic.co/elasticsearch/elasticsearch:8.13.4",
		tces.WithPassword(""),
		testcontainers.WithEnv(map[string]string{
			"discovery.type":         "single-node",
			"xpack.security.enabled": "false",
			"ES_JAVA_OPTS":           "-Xms512m -Xmx512m",
		}),
	)
	if err != nil {
		log.Fatalf("elasticsearch container: %v", err)
	}
	defer func() {
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		if err := esContainer.Terminate(termCtx); err != nil {
			log.Printf("terminate: %v", err)
		}
	}()

	addr := esContainer.Settings.Address
	if addr == "" {
		log.Fatalf("elasticsearch container address is empty")
	}

	client, err := es.New([]string{addr}, "", testIdx)
	if err != nil {
		log.Fatalf("es.New: %v", err)
	}
	testClient = client

	// Wait for the cluster to be ready, then create indices with refresh
	// disabled so we can call _refresh manually inside the test.
	if err := waitForGreen(ctx, addr); err != nil {
		log.Fatalf("waitForGreen: %v", err)
	}
	if err := testClient.EnsureIndices(ctx); err != nil {
		log.Fatalf("EnsureIndices: %v", err)
	}

	os.Exit(m.Run())
}

// waitForGreen polls the cluster health endpoint until it returns
// yellow/green or the context expires.
func waitForGreen(ctx context.Context, addr string) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		res, err := testClient.ES.Cluster.Health(testClient.ES.Cluster.Health.WithContext(ctx))
		if err == nil {
			res.Body.Close()
			if !res.IsError() {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("cluster never became reachable")
}

func refresh(t testing.TB) {
	t.Helper()
	res, err := testClient.ES.Indices.Refresh(testClient.ES.Indices.Refresh.WithIndex("_all"))
	require.NoError(t, err)
	defer res.Body.Close()
	require.Falsef(t, res.IsError(), "refresh failed: %s", res.String())
}

func deleteAllDocs(t testing.TB) {
	t.Helper()
	for _, idx := range []string{testIdx.Deck, testIdx.Folder, testIdx.Card, testIdx.User} {
		res, err := testClient.ES.DeleteByQuery(
			[]string{idx},
			strings.NewReader(`{"query":{"match_all":{}}}`),
			testClient.ES.DeleteByQuery.WithRefresh(true),
		)
		if err != nil {
			t.Fatalf("delete_by_query %s: %v", idx, err)
		}
		res.Body.Close()
	}
}

func TestSearchService_SearchDecks_ScopeFiltering(t *testing.T) {
	deleteAllDocs(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	svc := service.New(testClient)

	owner := uuid.New().String()
	other := uuid.New().String()

	docs := []es.DeckDoc{
		{
			DeckID: uuid.New().String(), UserID: owner,
			Name: "Italian Phrases", Description: "everyday talk",
			IsPublic: true, Status: "active", CardCount: 20,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			DeckID: uuid.New().String(), UserID: owner,
			Name: "My Private Italian Drills", Description: "personal notes",
			IsPublic: false, Status: "active", CardCount: 5,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			DeckID: uuid.New().String(), UserID: other,
			Name: "Public Spanish Vocab", Description: "spanish basic",
			IsPublic: true, Status: "active", CardCount: 30,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			DeckID: uuid.New().String(), UserID: other,
			Name: "Other User's Private Italian Notes", Description: "",
			IsPublic: false, Status: "active", CardCount: 1,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	for _, d := range docs {
		require.NoError(t, svc.IndexDeck(ctx, d))
	}
	refresh(t)

	t.Run("ScopePublic_AnonymousCaller", func(t *testing.T) {
		res, err := svc.SearchDecks(ctx, service.DeckSearchParams{
			Query: "italian", Scope: service.DeckScopePublic,
		})
		require.NoError(t, err)
		// Only the public Italian deck owned by `owner` should hit.
		require.EqualValues(t, 1, res.Total)
	})

	t.Run("ScopeMine_OnlyOwnerDecks", func(t *testing.T) {
		res, err := svc.SearchDecks(ctx, service.DeckSearchParams{
			Query: "italian", Scope: service.DeckScopeMine, CallerID: owner,
		})
		require.NoError(t, err)
		// Owner has 2 italian decks: public + private.
		require.EqualValues(t, 2, res.Total)
	})

	t.Run("ScopeMine_NoCaller_ReturnsEmpty", func(t *testing.T) {
		res, err := svc.SearchDecks(ctx, service.DeckSearchParams{
			Query: "italian", Scope: service.DeckScopeMine, CallerID: "",
		})
		require.NoError(t, err)
		assert.Empty(t, res.Hits)
	})

	t.Run("ScopeAll_PublicPlusOwn", func(t *testing.T) {
		res, err := svc.SearchDecks(ctx, service.DeckSearchParams{
			Query: "italian", Scope: service.DeckScopeAll, CallerID: owner,
		})
		require.NoError(t, err)
		// Owner sees: public Italian (theirs), private Italian (theirs) — 2.
		// `Other User's Private Italian Notes` must be hidden.
		require.EqualValues(t, 2, res.Total)
	})

	t.Run("EmptyQuery_MatchAll", func(t *testing.T) {
		res, err := svc.SearchDecks(ctx, service.DeckSearchParams{
			Query: "", Scope: service.DeckScopePublic,
		})
		require.NoError(t, err)
		// 2 public+active decks total.
		require.EqualValues(t, 2, res.Total)
	})
}

