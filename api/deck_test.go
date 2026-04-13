package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	mockdb "mem_pan/db/mock"
	db "mem_pan/db/sqlc"
)

func TestGetDeckAPI(t *testing.T) {
	ownerID := testUUID()
	deck := randomDeck(ownerID)
	otherUser := testUUID()

	testCases := []struct {
		name          string
		deckID        string
		setupAuth     func(request *http.Request)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "OK",
			deckID: deck.DeckID.String(),
			setupAuth: func(request *http.Request) {
				addAuthorization(request, ownerID)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetDeck(gomock.Any(), deck.DeckID).Times(1).Return(deck, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchDeck(t, recorder.Body, deck)
			},
		},
		{
			name:   "UnauthorizedUser",
			deckID: deck.DeckID.String(),
			setupAuth: func(request *http.Request) {
				addAuthorization(request, otherUser)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetDeck(gomock.Any(), deck.DeckID).Times(1).Return(deck, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			deckID:     deck.DeckID.String(),
			setupAuth:  func(request *http.Request) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:   "NotFound",
			deckID: deck.DeckID.String(),
			setupAuth: func(request *http.Request) {
				addAuthorization(request, ownerID)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetDeck(gomock.Any(), deck.DeckID).Times(1).Return(db.Deck{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:   "InvalidID",
			deckID: "bad-id",
			setupAuth: func(request *http.Request) {
				addAuthorization(request, ownerID)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/decks/%s", tc.deckID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(request)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
