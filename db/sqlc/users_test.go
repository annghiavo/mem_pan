package sqlc

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
	requireTestDB(t)

	arg := CreateUserParams{
		Username:     randomString("user"),
		Email:        randomEmail(),
		PasswordHash: randomString("passhash"),
		FullName:     sql.NullString{String: randomString("fullname"), Valid: true},
		AvatarUrl:    sql.NullString{String: "https://example.com/avatar.png", Valid: true},
		Role:         NullUserRole{UserRole: UserRoleUser, Valid: true},
	}

	user, err := testQueries.CreateUser(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, user)

	require.Equal(t, arg.Username, user.Username)
	require.Equal(t, arg.Email, user.Email)
	require.Equal(t, arg.PasswordHash, user.PasswordHash)
	require.Equal(t, arg.FullName, user.FullName)
	require.Equal(t, arg.AvatarUrl, user.AvatarUrl)
	require.Equal(t, arg.Role, user.Role)
	require.True(t, user.CreatedAt.Valid)
	require.NotEqual(t, user.UserID.String(), "")
}

func TestGetUserByID(t *testing.T) {
	requireTestDB(t)

	user1 := createRandomUser(t)
	user2, err := testQueries.GetUserByID(context.Background(), user1.UserID)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.UserID, user2.UserID)
	require.Equal(t, user1.Username, user2.Username)
	require.Equal(t, user1.Email, user2.Email)
	require.Equal(t, user1.Role, user2.Role)
	require.WithinDuration(t, user1.CreatedAt.Time, user2.CreatedAt.Time, time.Second)
}

func TestGetUserByEmail(t *testing.T) {
	requireTestDB(t)

	user1 := createRandomUser(t)
	user2, err := testQueries.GetUserByEmail(context.Background(), user1.Email)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.UserID, user2.UserID)
	require.Equal(t, user1.Email, user2.Email)
	require.Equal(t, user1.Username, user2.Username)
}

func TestUpdateUserProfile(t *testing.T) {
	requireTestDB(t)

	user1 := createRandomUser(t)
	lastLogin := time.Now().UTC().Truncate(time.Microsecond)

	arg := UpdateUserProfileParams{
		UserID:    user1.UserID,
		FullName:  sql.NullString{String: randomString("updated_name"), Valid: true},
		AvatarUrl: sql.NullString{String: "https://example.com/new-avatar.png", Valid: true},
		LastLogin: sql.NullTime{Time: lastLogin, Valid: true},
	}

	user2, err := testQueries.UpdateUserProfile(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.UserID, user2.UserID)
	require.Equal(t, user1.Username, user2.Username)
	require.Equal(t, user1.Email, user2.Email)
	require.Equal(t, arg.FullName, user2.FullName)
	require.Equal(t, arg.AvatarUrl, user2.AvatarUrl)
	require.True(t, user2.LastLogin.Valid)
	require.WithinDuration(t, arg.LastLogin.Time, user2.LastLogin.Time, time.Second)
}

func TestListUsers(t *testing.T) {
	requireTestDB(t)

	for i := 0; i < 5; i++ {
		createRandomUser(t)
	}

	users, err := testQueries.ListUsers(context.Background(), ListUsersParams{Limit: 5, Offset: 0})
	require.NoError(t, err)
	require.NotEmpty(t, users)

	for _, user := range users {
		require.NotEmpty(t, user)
		require.NotEqual(t, user.UserID.String(), "")
		require.NotEmpty(t, user.Username)
		require.NotEmpty(t, user.Email)
	}
}
