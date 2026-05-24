//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mem_pan/services/stats-service/internal/domain"
	"mem_pan/services/stats-service/internal/repository"
)

func TestStatsRepository_UserStatsLifecycle(t *testing.T) {
	truncateAll(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := repository.New(testDB)
	userID := uuid.New()

	// Create
	got, err := repo.CreateUserStats(ctx, userID, "annghiavo", "https://cdn/a.png")
	require.NoError(t, err)
	assert.Equal(t, userID, got.UserID)
	assert.Equal(t, "annghiavo", got.Username.String)
	assert.Equal(t, int32(0), got.CurrentStreak)

	// Round-trip Get
	fetched, err := repo.GetUserStats(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, got.UserID, fetched.UserID)
	assert.Equal(t, "annghiavo", fetched.Username.String)

	// Update streak — the date is stored at DATE precision; using midnight UTC.
	today := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpdateStreak(ctx, userID, 7, today))

	after, err := repo.GetUserStats(ctx, userID)
	require.NoError(t, err)
	assert.EqualValues(t, 7, after.CurrentStreak)
	require.True(t, after.LastStudiedDate.Valid)
	assert.Equal(t, today, after.LastStudiedDate.Time.UTC())
}

func TestStatsRepository_GetUserStats_NotFound(t *testing.T) {
	truncateAll(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repo := repository.New(testDB)
	_, err := repo.GetUserStats(ctx, uuid.New())
	require.ErrorIs(t, err, domain.ErrUserStatsNotFound)
}

func TestStatsRepository_BumpActivityBucket_AccumulatesOnUpsert(t *testing.T) {
	truncateAll(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repo := repository.New(testDB)
	userID := uuid.New()
	require.NoError(t, repo.BumpActivityBucket(ctx, userID, 19, 0, 3))
	require.NoError(t, repo.BumpActivityBucket(ctx, userID, 19, 0, 4))

	buckets, err := repo.ListActivityBuckets(ctx, userID)
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	assert.EqualValues(t, 19, buckets[0].HourOfDay)
	assert.EqualValues(t, 0, buckets[0].DayType)
	assert.EqualValues(t, 7, buckets[0].ReviewCount, "bumps must accumulate on PK conflict")
}
