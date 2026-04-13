package sqlc

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/require"
)

func randomString(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, rand.Int63())
}

func randomEmail() string {
	return fmt.Sprintf("%s@example.com", randomString("user"))
}

func randomBool() bool {
	return rand.Intn(2) == 1
}

func randomRole() UserRole {
	roles := []UserRole{UserRoleUser, UserRoleModerator, UserRoleAdmin}
	return roles[rand.Intn(len(roles))]
}

func randomDeckStatus() ContentStatus {
	statuses := []ContentStatus{ContentStatusActive, ContentStatusHidden, ContentStatusDeletedByAdmin}
	return statuses[rand.Intn(len(statuses))]
}

func randomCardState() CardState {
	states := []CardState{CardStateNew, CardStateLearning, CardStateReview, CardStateRelearning}
	return states[rand.Intn(len(states))]
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func createRandomUser(t *testing.T) User {
	t.Helper()
	requireTestDB(t)

	arg := CreateUserParams{
		Username:     randomString("user"),
		Email:        randomEmail(),
		PasswordHash: randomString("passhash"),
		FullName:     sql.NullString{String: randomString("name"), Valid: true},
		AvatarUrl:    sql.NullString{String: "https://example.com/avatar.png", Valid: true},
		Role:         NullUserRole{UserRole: randomRole(), Valid: true},
	}

	user, err := testQueries.CreateUser(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, user)
	return user
}

func createRandomDeck(t *testing.T, userID uuid.UUID) Deck {
	t.Helper()
	requireTestDB(t)

	settings := pqtype.NullRawMessage{RawMessage: []byte(`{"quiz_type":"typing"}`), Valid: true}
	arg := CreateDeckParams{
		UserID:      userID,
		Name:        randomString("deck"),
		Description: sql.NullString{String: randomString("desc"), Valid: true},
		IsPublic:    sql.NullBool{Bool: randomBool(), Valid: true},
		Status:      NullContentStatus{ContentStatus: randomDeckStatus(), Valid: true},
		Settings:    settings,
	}

	deck, err := testQueries.CreateDeck(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, deck)
	return deck
}

func createRandomNote(t *testing.T, userID uuid.UUID) uuid.UUID {
	t.Helper()
	requireTestDB(t)

	var noteID uuid.UUID
	err := testDB.QueryRowContext(
		context.Background(),
		`INSERT INTO notes (user_id, content_front, content_back)
		 VALUES ($1, $2, $3)
		 RETURNING note_id`,
		userID,
		randomString("front"),
		randomString("back"),
	).Scan(&noteID)
	require.NoError(t, err)
	return noteID
}

func createRandomCard(t *testing.T, userID, deckID, noteID uuid.UUID) Card {
	t.Helper()
	requireTestDB(t)

	arg := CreateCardParams{
		UserID: userID,
		NoteID: noteID,
		DeckID: deckID,
		State:  NullCardState{CardState: randomCardState(), Valid: true},
		NextReviewDate: sql.NullTime{
			Time:  time.Now().Add(12 * time.Hour),
			Valid: true,
		},
	}

	card, err := testQueries.CreateCard(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, card)
	return card
}

func createRandomRevlog(t *testing.T, cardID, userID uuid.UUID) Revlog {
	t.Helper()
	requireTestDB(t)

	arg := CreateRevlogParams{
		CardID:      cardID,
		UserID:      userID,
		Rating:      int16(1 + rand.Intn(4)),
		DurationMs:  int32(500 + rand.Intn(3000)),
		State:       randomCardState(),
		ElapsedDays: int32(rand.Intn(30)),
		StabilityBefore: sql.NullFloat64{
			Float64: 0.8 + rand.Float64()*4,
			Valid:   true,
		},
		DifficultyBefore: sql.NullFloat64{
			Float64: 2 + rand.Float64()*5,
			Valid:   true,
		},
	}

	revlog, err := testQueries.CreateRevlog(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, revlog)
	return revlog
}
