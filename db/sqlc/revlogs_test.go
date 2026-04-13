package sqlc

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateRevlog(t *testing.T) {
	requireTestDB(t)

	user := createRandomUser(t)
	deck := createRandomDeck(t, user.UserID)
	noteID := createRandomNote(t, user.UserID)
	card := createRandomCard(t, user.UserID, deck.DeckID, noteID)

	arg := CreateRevlogParams{
		CardID:      card.CardID,
		UserID:      user.UserID,
		Rating:      3,
		DurationMs:  1200,
		State:       CardStateReview,
		ElapsedDays: 7,
		StabilityBefore: sql.NullFloat64{
			Float64: 3.2,
			Valid:   true,
		},
		DifficultyBefore: sql.NullFloat64{
			Float64: 5.1,
			Valid:   true,
		},
	}

	revlog, err := testQueries.CreateRevlog(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, revlog)

	require.Equal(t, arg.CardID, revlog.CardID)
	require.Equal(t, arg.UserID, revlog.UserID)
	require.Equal(t, arg.Rating, revlog.Rating)
	require.Equal(t, arg.DurationMs, revlog.DurationMs)
	require.Equal(t, arg.State, revlog.State)
	require.Equal(t, arg.ElapsedDays, revlog.ElapsedDays)
	require.Equal(t, arg.StabilityBefore, revlog.StabilityBefore)
	require.Equal(t, arg.DifficultyBefore, revlog.DifficultyBefore)
	require.True(t, revlog.ReviewTime.Valid)
	require.NotEqual(t, revlog.LogID.String(), "")
}

func TestListRevlogsByCard(t *testing.T) {
	requireTestDB(t)

	user := createRandomUser(t)
	deck := createRandomDeck(t, user.UserID)
	noteID := createRandomNote(t, user.UserID)
	card := createRandomCard(t, user.UserID, deck.DeckID, noteID)

	var first Revlog
	for i := 0; i < 4; i++ {
		r := createRandomRevlog(t, card.CardID, user.UserID)
		if i == 0 {
			first = r
		}
		time.Sleep(5 * time.Millisecond)
	}

	otherUser := createRandomUser(t)
	otherDeck := createRandomDeck(t, otherUser.UserID)
	otherNoteID := createRandomNote(t, otherUser.UserID)
	otherCard := createRandomCard(t, otherUser.UserID, otherDeck.DeckID, otherNoteID)
	createRandomRevlog(t, otherCard.CardID, otherUser.UserID)

	revlogs, err := testQueries.ListRevlogsByCard(context.Background(), ListRevlogsByCardParams{
		CardID: card.CardID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, revlogs)

	for _, revlog := range revlogs {
		require.NotEmpty(t, revlog)
		require.Equal(t, card.CardID, revlog.CardID)
	}

	require.Equal(t, first.CardID, revlogs[len(revlogs)-1].CardID)
}

func TestListRevlogsByUser(t *testing.T) {
	requireTestDB(t)

	user := createRandomUser(t)
	deck := createRandomDeck(t, user.UserID)
	noteID := createRandomNote(t, user.UserID)
	card := createRandomCard(t, user.UserID, deck.DeckID, noteID)

	for i := 0; i < 5; i++ {
		createRandomRevlog(t, card.CardID, user.UserID)
	}

	otherUser := createRandomUser(t)
	otherDeck := createRandomDeck(t, otherUser.UserID)
	otherNoteID := createRandomNote(t, otherUser.UserID)
	otherCard := createRandomCard(t, otherUser.UserID, otherDeck.DeckID, otherNoteID)
	createRandomRevlog(t, otherCard.CardID, otherUser.UserID)

	revlogs, err := testQueries.ListRevlogsByUser(context.Background(), ListRevlogsByUserParams{
		UserID: user.UserID,
		Limit:  5,
		Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, revlogs)

	for _, revlog := range revlogs {
		require.NotEmpty(t, revlog)
		require.Equal(t, user.UserID, revlog.UserID)
	}
}
