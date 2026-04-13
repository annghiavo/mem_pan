package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/require"

	db "mem_pan/db/sqlc"
)

func newTestServer(t *testing.T, store Store) *Server {
	ginSetTestMode()
	server := NewServerWithStore(nil, store)
	require.NotNil(t, server)
	return server
}

func addAuthorization(request *http.Request, userID uuid.UUID) {
	request.Header.Set("x-user-id", userID.String())
}

func ginSetTestMode() {
	gin.SetMode(gin.TestMode)
}

func testUUID() uuid.UUID {
	return uuid.New()
}

func randomDeck(ownerID uuid.UUID) db.Deck {
	return db.Deck{
		DeckID:   uuid.New(),
		UserID:   ownerID,
		Name:     "deck-" + uuid.NewString()[:8],
		IsPublic: sql.NullBool{Bool: false, Valid: true},
		Status: db.NullContentStatus{
			ContentStatus: db.ContentStatusActive,
			Valid:         true,
		},
		Settings: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"quiz_type":"multiple_choice"}`), Valid: true},
	}
}

func requireBodyMatchDeck(t *testing.T, body *bytes.Buffer, deck db.Deck) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var got db.Deck
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)
	require.Equal(t, deck.DeckID, got.DeckID)
	require.Equal(t, deck.UserID, got.UserID)
	require.Equal(t, deck.Name, got.Name)
}

func requireBodyMatchDecks(t *testing.T, body *bytes.Buffer, decks []db.Deck) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var got []db.Deck
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)
	require.Len(t, got, len(decks))
	for i := range decks {
		require.Equal(t, decks[i].DeckID, got[i].DeckID)
		require.Equal(t, decks[i].Name, got[i].Name)
	}
}

func mustJSONBody(t *testing.T, v any) *strings.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return strings.NewReader(string(b))
}

func newJSONRequest(t *testing.T, method, url string, body any) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = mustJSONBody(t, body)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func requireErrorContains(t *testing.T, recorder *httptest.ResponseRecorder, code int, contains string) {
	t.Helper()
	require.Equal(t, code, recorder.Code)
	var payload map[string]string
	err := json.Unmarshal(recorder.Body.Bytes(), &payload)
	require.NoError(t, err)
	require.Contains(t, payload["error"], contains, fmt.Sprintf("payload=%v", payload))
}
