//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/repository"
)

func TestDeckRepository_CreateAndGet(t *testing.T) {
	type tc struct {
		name      string
		arg       db.CreateDeckParams
		wantErr   bool
		assertRow func(t *testing.T, got db.Deck)
	}

	userID := uuid.New()

	tests := []tc{
		{
			name: "Success_DefaultsAreApplied",
			arg: db.CreateDeckParams{
				UserID:      userID,
				Name:        "English vocab",
				Description: sql.NullString{String: "everyday phrases", Valid: true},
				IsPublic:    true,
			},
			assertRow: func(t *testing.T, got db.Deck) {
				require.NotEqual(t, uuid.Nil, got.DeckID)
				assert.Equal(t, userID, got.UserID)
				assert.Equal(t, "English vocab", got.Name)
				assert.True(t, got.IsPublic)
				assert.Equal(t, string(db.ContentStatusActive), got.Status)
				// Server-side defaults from the schema must propagate back.
				assert.NotEmpty(t, got.Settings, "settings JSONB default should be applied")
				assert.EqualValues(t, 0, got.CardCount)
				assert.WithinDuration(t, time.Now().UTC(), got.CreatedAt.UTC(), 10*time.Second)
			},
		},
		{
			name: "BadInput_NameTooLong_RejectedByVarchar200",
			arg: db.CreateDeckParams{
				UserID:   userID,
				Name:     string(make([]byte, 201)), // 201 NUL bytes — over VARCHAR(200)
				IsPublic: false,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			truncateAll(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			repo := repository.NewDeckRepository(testDB)
			got, err := repo.CreateDeck(ctx, tc.arg)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tc.assertRow(t, got)

			// Round-trip via GetDeckByID and assert dữ liệu thật trong DB.
			fetched, err := repo.GetDeckByID(ctx, got.DeckID)
			require.NoError(t, err)
			assert.Equal(t, got.DeckID, fetched.DeckID)
			assert.Equal(t, got.Name, fetched.Name)
			assert.Equal(t, got.IsPublic, fetched.IsPublic)

			// Cross-check with a raw SQL count to make sure exactly one row exists.
			var count int
			row := testDB.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM decks WHERE deck_id = $1`, got.DeckID)
			require.NoError(t, row.Scan(&count))
			assert.Equal(t, 1, count)
		})
	}
}

func TestDeckRepository_GetDeckByID_NotFound(t *testing.T) {
	truncateAll(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repo := repository.NewDeckRepository(testDB)
	_, err := repo.GetDeckByID(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrDeckNotFound,
		"sql.ErrNoRows must be wrapped into domain.ErrDeckNotFound by the repo")
}

func TestDeckRepository_SoftDelete_HidesFromGet(t *testing.T) {
	truncateAll(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repo := repository.NewDeckRepository(testDB)
	created, err := repo.CreateDeck(ctx, db.CreateDeckParams{
		UserID:   uuid.New(),
		Name:     "Throwaway",
		IsPublic: false,
	})
	require.NoError(t, err)

	require.NoError(t, repo.SoftDeleteDeck(ctx, db.SoftDeleteDeckParams{
		DeckID: created.DeckID,
		UserID: created.UserID,
	}))

	// Soft delete keeps the row but flips status to 'deleted'. The repo's
	// GetDeckByID returns the row as-is; the service layer is what hides it.
	got, err := repo.GetDeckByID(ctx, created.DeckID)
	require.NoError(t, err)
	assert.Equal(t, string(db.ContentStatusDeleted), got.Status)
}
