//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mem_pan/services/notification-service/internal/db"
	"mem_pan/services/notification-service/internal/repository"
)

func TestNotificationRepository_FCMTokenLifecycle(t *testing.T) {
	truncateAll(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := repository.New(testDB)
	userID := uuid.New()
	const tokA = "fcm-token-A"
	const tokB = "fcm-token-B"

	// Upsert two distinct tokens for the same user.
	got, err := repo.UpsertFCMToken(ctx, userID, tokA, "Pixel 8")
	require.NoError(t, err)
	assert.Equal(t, tokA, got.Token)
	assert.Equal(t, "Pixel 8", got.DeviceName)

	_, err = repo.UpsertFCMToken(ctx, userID, tokB, "iPad")
	require.NoError(t, err)

	tokens, err := repo.ListFCMTokensByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, tokens, 2)

	// Upsert again with same token: must not duplicate.
	_, err = repo.UpsertFCMToken(ctx, userID, tokA, "Pixel 8 (renamed)")
	require.NoError(t, err)

	tokens, err = repo.ListFCMTokensByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, tokens, 2, "duplicate token must update in-place")

	// Delete one then verify.
	require.NoError(t, repo.DeleteFCMToken(ctx, userID, tokA))

	tokens, err = repo.ListFCMTokensByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, tokB, tokens[0].Token)
}

func TestNotificationRepository_CountRecentNotifications(t *testing.T) {
	truncateAll(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := repository.New(testDB)
	userID := uuid.New()
	uid := userID

	// Insert two "sent" log rows + one "failed" row for the same type.
	for i := 0; i < 2; i++ {
		require.NoError(t, repo.LogNotification(ctx, db.LogNotificationParams{
			UserID:           &uid,
			NotificationType: "study_reminder",
			Channel:          "fcm",
			Recipient:        userID.String(),
			Status:           "sent",
		}))
	}
	failMsg := "boom"
	require.NoError(t, repo.LogNotification(ctx, db.LogNotificationParams{
		UserID:           &uid,
		NotificationType: "study_reminder",
		Channel:          "fcm",
		Recipient:        userID.String(),
		Status:           "failed",
		ErrorMessage:     &failMsg,
	}))

	count, err := repo.CountRecentNotifications(ctx, userID, "study_reminder",
		time.Now().Add(-1*time.Hour))
	require.NoError(t, err)
	// The exact filter (sent vs failed) is up to the SQL — we just assert
	// that *at least* the 2 sent rows are counted within the window.
	assert.GreaterOrEqual(t, count, int64(2))

	// Window in the future: nothing recent.
	count, err = repo.CountRecentNotifications(ctx, userID, "study_reminder",
		time.Now().Add(1*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
