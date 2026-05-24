package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"mem_pan/services/stats-service/internal/db"
	"mem_pan/services/stats-service/internal/mock"
)

// hourPtr is a tiny helper so test cases can express expected optional hours
// in literal form: hourPtr(19) instead of declaring a temp variable each time.
func hourPtr(v int16) *int16 { return &v }

func TestStatsService_RecomputeOptimalHours(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	errDB := errors.New("connection refused")

	tests := []struct {
		name      string
		buckets   []db.UserActivityBucket
		repoErr   error
		setErr    error
		wantWD    *int16
		wantWE    *int16
		wantErr   error
		skipSet   bool
	}{
		{
			name: "Success_ArgmaxWritesBothDayTypes",
			buckets: []db.UserActivityBucket{
				{UserID: userID, HourOfDay: 7, DayType: 0, ReviewCount: 2},
				{UserID: userID, HourOfDay: 19, DayType: 0, ReviewCount: 8}, // argmax weekday
				{UserID: userID, HourOfDay: 9, DayType: 1, ReviewCount: 10}, // argmax weekend
				{UserID: userID, HourOfDay: 21, DayType: 1, ReviewCount: 3},
			},
			wantWD: hourPtr(19),
			wantWE: hourPtr(9),
		},
		{
			name: "Success_WeekdayBelowMinSamples_StaysNull",
			buckets: []db.UserActivityBucket{
				// Weekday: 4 < minSamples(5) → must NOT set.
				{UserID: userID, HourOfDay: 6, DayType: 0, ReviewCount: 4},
				// Weekend: 5 == minSamples → set.
				{UserID: userID, HourOfDay: 22, DayType: 1, ReviewCount: 5},
			},
			wantWD: nil,
			wantWE: hourPtr(22),
		},
		{
			name:    "Success_NoBuckets_NoSet_OnlyEmptyArgs",
			buckets: nil,
			wantWD:  nil,
			wantWE:  nil,
		},
		{
			name: "Success_IgnoresInvalidDayType",
			buckets: []db.UserActivityBucket{
				// DayType outside [0,1] is dropped silently.
				{UserID: userID, HourOfDay: 5, DayType: 2, ReviewCount: 100},
				{UserID: userID, HourOfDay: 19, DayType: 0, ReviewCount: 6},
			},
			wantWD: hourPtr(19),
			wantWE: nil,
		},
		{
			name:    "InternalError_ListActivityBuckets_DBDown",
			buckets: nil,
			repoErr: errDB,
			wantErr: errDB,
			skipSet: true,
		},
		{
			name: "InternalError_SetOptimalHours_Propagates",
			buckets: []db.UserActivityBucket{
				{UserID: userID, HourOfDay: 8, DayType: 0, ReviewCount: 6},
			},
			setErr:  errDB,
			wantWD:  hourPtr(8),
			wantErr: errDB,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := mock.NewMockStatsRepository(ctrl)

			repo.EXPECT().
				ListActivityBuckets(gomock.Any(), userID).
				Return(tc.buckets, tc.repoErr)

			if !tc.skipSet {
				// Assert SetOptimalHours is called with EXACTLY the expected
				// nullable hour pointers.
				repo.EXPECT().
					SetOptimalHours(gomock.Any(), userID,
						gomock.AssignableToTypeOf((*int16)(nil)),
						gomock.AssignableToTypeOf((*int16)(nil))).
					DoAndReturn(func(_ context.Context, _ uuid.UUID, wd, we *int16) error {
						if tc.wantWD == nil {
							require.Nil(t, wd, "weekday must be nil")
						} else {
							require.NotNil(t, wd)
							require.Equal(t, *tc.wantWD, *wd)
						}
						if tc.wantWE == nil {
							require.Nil(t, we, "weekend must be nil")
						} else {
							require.NotNil(t, we)
							require.Equal(t, *tc.wantWE, *we)
						}
						return tc.setErr
					}).Times(1)
			}

			svc := New(repo)
			err := svc.RecomputeOptimalHours(context.Background(), userID)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
