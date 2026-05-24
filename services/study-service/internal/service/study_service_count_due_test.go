package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"mem_pan/services/study-service/internal/mock"
)

// TestCountDueByEndOfDay locks in the timezone-handling contract:
//   - blank tz   → fall back to UTC silently.
//   - junk tz    → fall back to UTC silently.
//   - real tz    → upper bound passed to repo equals end-of-local-day, in UTC.
//
// The cron worker that consumes this value depends on the UTC-conversion
// being correct; a regression here would silently send reminders at the
// wrong wall-clock time.
func TestStudyService_CountDueByEndOfDay(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	errDB := errors.New("connection refused")

	tests := []struct {
		name     string
		timezone string
		repoErr  error
		repoCnt  int32
		wantErr  error
		wantCnt  int32
		// loc is what time.LoadLocation(timezone) is expected to yield; the
		// test rebuilds the expected UTC end-of-day inside DoAndReturn so
		// it stays correct as time marches on.
		expectedLoc func() *time.Location
	}{
		{
			name:        "Success_UTC_Explicit",
			timezone:    "UTC",
			repoCnt:     7,
			wantCnt:     7,
			expectedLoc: func() *time.Location { return time.UTC },
		},
		{
			name:        "Success_EmptyTZ_FallsBackToUTC",
			timezone:    "",
			repoCnt:     3,
			wantCnt:     3,
			expectedLoc: func() *time.Location { return time.UTC },
		},
		{
			name:        "Success_InvalidTZ_FallsBackToUTC",
			timezone:    "Mars/Olympus",
			repoCnt:     0,
			wantCnt:     0,
			expectedLoc: func() *time.Location { return time.UTC },
		},
		{
			name:     "Success_HoChiMinhTZ_UpperBoundComputedFromLocalEOD",
			timezone: "Asia/Ho_Chi_Minh",
			repoCnt:  12,
			wantCnt:  12,
			expectedLoc: func() *time.Location {
				loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
				return loc
			},
		},
		{
			name:    "InternalError_DBDown_PropagatesError",
			timezone: "UTC",
			repoErr:  errDB,
			wantErr:  errDB,
			expectedLoc: func() *time.Location { return time.UTC },
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			ucRepo := mock.NewMockUserCardRepository(ctrl)
			sessRepo := mock.NewMockStudySessionRepository(ctrl)
			scRepo := mock.NewMockSessionCardRepository(ctrl)
			revRepo := mock.NewMockRevlogRepository(ctrl)
			weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
			deckClient := mock.NewMockDeckClient(ctrl)

			ucRepo.EXPECT().
				CountDueByEndOfDay(gomock.Any(), userID, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ uuid.UUID, endUTC time.Time) (int32, error) {
					// endUTC must equal end-of-day in the resolved location, then UTC.
					loc := tc.expectedLoc()
					now := time.Now().In(loc)
					wantEnd := time.Date(now.Year(), now.Month(), now.Day(),
						23, 59, 59, int(time.Second-time.Nanosecond), loc).UTC()

					// Allow ±2s drift because time.Now() advances between
					// service call and our recomputation here.
					require.WithinDuration(t, wantEnd, endUTC, 2*time.Second,
						"end-of-day bound must be local 23:59:59 then UTC")
					return tc.repoCnt, tc.repoErr
				}).
				Times(1)

			svc := NewStudyService(ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
			got, err := svc.CountDueByEndOfDay(context.Background(), userID, tc.timezone)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantCnt, got)
		})
	}
}
