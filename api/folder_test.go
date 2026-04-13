package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	mockdb "mem_pan/db/mock"
	db "mem_pan/db/sqlc"
)

func TestListFoldersAPI(t *testing.T) {
	userID := uuid.New()
	folderID := uuid.New()
	deck := randomDeck(userID)
	folders := []db.Folder{{FolderID: folderID, UserID: userID, Name: "folder-1"}}

	testCases := []struct {
		name          string
		pageID        int
		pageSize      int
		setupAuth     func(request *http.Request)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			pageID:   1,
			pageSize: 5,
			setupAuth: func(request *http.Request) {
				addAuthorization(request, userID)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListFoldersByUser(gomock.Any(), gomock.Any()).Times(1).Return(folders, nil)
				store.EXPECT().ListDecksInFolder(gomock.Any(), folderID).Times(1).Return([]db.Deck{deck}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			pageID:     1,
			pageSize:   5,
			setupAuth:  func(request *http.Request) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:     "InvalidPageSize",
			pageID:   1,
			pageSize: 1000,
			setupAuth: func(request *http.Request) {
				addAuthorization(request, userID)
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

			url := fmt.Sprintf("/folders?page_id=%d&page_size=%d", tc.pageID, tc.pageSize)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(request)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
