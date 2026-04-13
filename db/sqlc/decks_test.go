package sqlc

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/require"
)

func TestCreateDeck(t *testing.T) {
	requireTestDB(t)

	owner := createRandomUser(t)
	arg := CreateDeckParams{
		UserID:      owner.UserID,
		Name:        randomString("deck"),
		Description: sql.NullString{String: randomString("desc"), Valid: true},
		IsPublic:    sql.NullBool{Bool: true, Valid: true},
		Status:      NullContentStatus{ContentStatus: ContentStatusActive, Valid: true},
		Settings: pqtype.NullRawMessage{
			RawMessage: []byte(`{"quiz_type":"multiple_choice","answer_side":"back"}`),
			Valid:      true,
		},
	}

	deck, err := testQueries.CreateDeck(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, deck)

	require.Equal(t, arg.UserID, deck.UserID)
	require.Equal(t, arg.Name, deck.Name)
	require.Equal(t, arg.Description, deck.Description)
	require.Equal(t, arg.IsPublic, deck.IsPublic)
	require.Equal(t, arg.Status, deck.Status)
	require.JSONEq(t, string(arg.Settings.RawMessage), string(deck.Settings.RawMessage))
	require.True(t, deck.CreatedAt.Valid)
	require.NotEqual(t, deck.DeckID.String(), "")
}

func TestGetDeck(t *testing.T) {
	requireTestDB(t)

	owner := createRandomUser(t)
	deck1 := createRandomDeck(t, owner.UserID)

	deck2, err := testQueries.GetDeck(context.Background(), deck1.DeckID)
	require.NoError(t, err)
	require.NotEmpty(t, deck2)

	require.Equal(t, deck1.DeckID, deck2.DeckID)
	require.Equal(t, deck1.UserID, deck2.UserID)
	require.Equal(t, deck1.Name, deck2.Name)
	require.WithinDuration(t, deck1.CreatedAt.Time, deck2.CreatedAt.Time, time.Second)
}

func TestUpdateDeck(t *testing.T) {
	requireTestDB(t)

	owner := createRandomUser(t)
	deck1 := createRandomDeck(t, owner.UserID)

	arg := UpdateDeckParams{
		DeckID:      deck1.DeckID,
		Name:        sql.NullString{String: randomString("updated_deck"), Valid: true},
		Description: sql.NullString{String: randomString("updated_desc"), Valid: true},
		IsPublic:    sql.NullBool{Bool: !deck1.IsPublic.Bool, Valid: true},
		Status:      NullContentStatus{ContentStatus: ContentStatusHidden, Valid: true},
		Settings: pqtype.NullRawMessage{
			RawMessage: []byte(`{"quiz_type":"typing","strict_typing":true}`),
			Valid:      true,
		},
	}

	deck2, err := testQueries.UpdateDeck(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, deck2)

	require.Equal(t, deck1.DeckID, deck2.DeckID)
	require.Equal(t, deck1.UserID, deck2.UserID)
	require.Equal(t, arg.Name.String, deck2.Name)
	require.Equal(t, arg.Description, deck2.Description)
	require.Equal(t, arg.IsPublic, deck2.IsPublic)
	require.Equal(t, arg.Status, deck2.Status)
	require.JSONEq(t, string(arg.Settings.RawMessage), string(deck2.Settings.RawMessage))
	require.True(t, deck2.UpdatedAt.Valid)
}

func TestListUserDecks(t *testing.T) {
	requireTestDB(t)

	owner := createRandomUser(t)
	for i := 0; i < 6; i++ {
		createRandomDeck(t, owner.UserID)
	}
	other := createRandomUser(t)
	createRandomDeck(t, other.UserID)

	decks, err := testQueries.ListUserDecks(context.Background(), ListUserDecksParams{
		UserID: owner.UserID,
		Limit:  5,
		Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, decks)

	for _, deck := range decks {
		require.NotEmpty(t, deck)
		require.Equal(t, owner.UserID, deck.UserID)
	}
}

func TestListPublicActiveDecks(t *testing.T) {
	requireTestDB(t)

	owner := createRandomUser(t)

	for i := 0; i < 3; i++ {
		_, err := testQueries.CreateDeck(context.Background(), CreateDeckParams{
			UserID:      owner.UserID,
			Name:        randomString("public_active"),
			Description: sql.NullString{String: randomString("desc"), Valid: true},
			IsPublic:    sql.NullBool{Bool: true, Valid: true},
			Status:      NullContentStatus{ContentStatus: ContentStatusActive, Valid: true},
			Settings:    pqtype.NullRawMessage{RawMessage: []byte(`{"quiz_type":"multiple_choice"}`), Valid: true},
		})
		require.NoError(t, err)
	}

	_, err := testQueries.CreateDeck(context.Background(), CreateDeckParams{
		UserID:      owner.UserID,
		Name:        randomString("private_active"),
		Description: sql.NullString{String: randomString("desc"), Valid: true},
		IsPublic:    sql.NullBool{Bool: false, Valid: true},
		Status:      NullContentStatus{ContentStatus: ContentStatusActive, Valid: true},
		Settings:    pqtype.NullRawMessage{RawMessage: []byte(`{"quiz_type":"multiple_choice"}`), Valid: true},
	})
	require.NoError(t, err)

	_, err = testQueries.CreateDeck(context.Background(), CreateDeckParams{
		UserID:      owner.UserID,
		Name:        randomString("public_hidden"),
		Description: sql.NullString{String: randomString("desc"), Valid: true},
		IsPublic:    sql.NullBool{Bool: true, Valid: true},
		Status:      NullContentStatus{ContentStatus: ContentStatusHidden, Valid: true},
		Settings:    pqtype.NullRawMessage{RawMessage: []byte(`{"quiz_type":"multiple_choice"}`), Valid: true},
	})
	require.NoError(t, err)

	decks, err := testQueries.ListPublicActiveDecks(context.Background(), ListPublicActiveDecksParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.NotEmpty(t, decks)

	for _, deck := range decks {
		require.True(t, deck.IsPublic.Valid)
		require.True(t, deck.IsPublic.Bool)
		require.True(t, deck.Status.Valid)
		require.Equal(t, ContentStatusActive, deck.Status.ContentStatus)
	}
}
