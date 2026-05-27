package subscriber

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"mem_pan/services/stats-service/internal/db"
	"mem_pan/services/stats-service/internal/domain"
	"mem_pan/services/stats-service/internal/events"
	"mem_pan/services/stats-service/internal/mock"
)

// dispatchDeckDeleted marshals a DeckDeleted payload and routes it through the
// public Dispatch entry point, exercising both the event registration and the
// handler.
func dispatchDeckDeleted(t *testing.T, h *Handler, deckID, userID string) error {
	t.Helper()
	data, err := json.Marshal(events.DeckDeleted{DeckID: deckID, UserID: userID})
	require.NoError(t, err)
	return h.Dispatch(context.Background(), events.TypeDeckDeleted, data)
}

func TestHandler_DeckDeleted(t *testing.T) {
	t.Parallel()

	deckID := uuid.New()
	userID := uuid.New()
	errDB := errors.New("connection refused")

	t.Run("Success_DeletesAggregates_DecrementsUserCards", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockStatsRepository(ctrl)

		// Card count is read from deck_stats, then subtracted from the user's
		// lifetime total before the deck-scoped rows are removed.
		repo.EXPECT().GetDeckStats(gomock.Any(), deckID).
			Return(db.DeckStat{DeckID: deckID, UserID: userID, TotalCards: 5}, nil)
		repo.EXPECT().DecrementUserCards(gomock.Any(), userID, int32(5)).Return(nil)
		repo.EXPECT().DeleteDeckProgressSnapshots(gomock.Any(), deckID, userID).Return(nil)
		repo.EXPECT().DeleteDeckStats(gomock.Any(), deckID, userID).Return(nil)

		err := dispatchDeckDeleted(t, NewHandler(repo), deckID.String(), userID.String())
		require.NoError(t, err)
	})

	t.Run("Success_DeckStatsMissing_SkipsDecrement", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockStatsRepository(ctrl)

		// Duplicate delivery (or a deck created before stats-service existed):
		// no deck_stats row, so the decrement is skipped but the DELETEs still
		// run as harmless no-ops. DecrementUserCards must NOT be called.
		repo.EXPECT().GetDeckStats(gomock.Any(), deckID).
			Return(db.DeckStat{}, domain.ErrDeckStatsNotFound)
		repo.EXPECT().DeleteDeckProgressSnapshots(gomock.Any(), deckID, userID).Return(nil)
		repo.EXPECT().DeleteDeckStats(gomock.Any(), deckID, userID).Return(nil)

		err := dispatchDeckDeleted(t, NewHandler(repo), deckID.String(), userID.String())
		require.NoError(t, err)
	})

	t.Run("Success_ZeroCards_SkipsDecrement", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockStatsRepository(ctrl)

		// An empty deck has nothing to subtract — DecrementUserCards must NOT be
		// called (calling it with 0 would be a wasted write).
		repo.EXPECT().GetDeckStats(gomock.Any(), deckID).
			Return(db.DeckStat{DeckID: deckID, UserID: userID, TotalCards: 0}, nil)
		repo.EXPECT().DeleteDeckProgressSnapshots(gomock.Any(), deckID, userID).Return(nil)
		repo.EXPECT().DeleteDeckStats(gomock.Any(), deckID, userID).Return(nil)

		err := dispatchDeckDeleted(t, NewHandler(repo), deckID.String(), userID.String())
		require.NoError(t, err)
	})

	t.Run("Error_GetDeckStatsFails_AbortsBeforeAnyDelete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockStatsRepository(ctrl)

		// A genuine DB error (not "not found") must propagate so Pub/Sub retries;
		// nothing should be deleted in that case.
		repo.EXPECT().GetDeckStats(gomock.Any(), deckID).Return(db.DeckStat{}, errDB)

		err := dispatchDeckDeleted(t, NewHandler(repo), deckID.String(), userID.String())
		require.ErrorIs(t, err, errDB)
	})

	t.Run("Error_DecrementFails_Propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockStatsRepository(ctrl)

		repo.EXPECT().GetDeckStats(gomock.Any(), deckID).
			Return(db.DeckStat{DeckID: deckID, UserID: userID, TotalCards: 3}, nil)
		repo.EXPECT().DecrementUserCards(gomock.Any(), userID, int32(3)).Return(errDB)

		err := dispatchDeckDeleted(t, NewHandler(repo), deckID.String(), userID.String())
		require.ErrorIs(t, err, errDB)
	})

	t.Run("Error_InvalidDeckID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockStatsRepository(ctrl)

		// Malformed payload must fail before touching the repository.
		err := dispatchDeckDeleted(t, NewHandler(repo), "not-a-uuid", userID.String())
		require.Error(t, err)
	})

	t.Run("Error_InvalidUserID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mock.NewMockStatsRepository(ctrl)

		err := dispatchDeckDeleted(t, NewHandler(repo), deckID.String(), "not-a-uuid")
		require.Error(t, err)
	})
}
