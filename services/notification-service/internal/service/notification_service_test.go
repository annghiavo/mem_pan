package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"mem_pan/services/notification-service/internal/db"
	"mem_pan/services/notification-service/internal/mock"
)

// TestNotificationService_SendDeckCloneReadyPush exercises the four branches
// of the push pipeline: invalid UUID, repo error, empty device list (skip),
// happy path (asserts FCM payload), and FCM transport error.
func TestNotificationService_SendDeckCloneReadyPush(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	deckID := uuid.New().String()
	const deckName = "Italian Greetings"
	const cardCount int32 = 12

	errDB := errors.New("connection refused")
	errFCM := errors.New("fcm: 503 unavailable")

	type deps struct {
		repo *mock.MockNotificationRepository
		fcm  *mock.MockFCMSender
	}

	tests := []struct {
		name            string
		userID          string
		setup           func(d deps)
		wantErr         error
		wantInvalidUUID bool
	}{
		{
			name:   "BadInput_InvalidUUID_NoRepoOrFCM",
			userID: "not-a-uuid",
			setup: func(d deps) {
				// No mock calls expected — controller fails the test if any
				// mock method runs without a matching EXPECT().
			},
			wantErr: nil, // error is uuid.Parse, not errDB — assert error path below
			wantInvalidUUID: true,
		},
		{
			name:   "InternalError_ListTokens_DBDown",
			userID: validUser,
			setup: func(d deps) {
				d.repo.EXPECT().
					ListFCMTokensByUser(gomock.Any(), gomock.Any()).
					Return(nil, errDB)
				// FCM must NOT be called when token lookup fails.
			},
			wantErr: errDB,
		},
		{
			name:   "Success_NoTokens_EarlyReturn",
			userID: validUser,
			setup: func(d deps) {
				d.repo.EXPECT().
					ListFCMTokensByUser(gomock.Any(), gomock.Any()).
					Return([]db.FcmToken{}, nil)
				// FCM must NOT be called, no LogNotification either.
			},
		},
		{
			name:   "Success_FCMSent_LogsWithCorrectPayload",
			userID: validUser,
			setup: func(d deps) {
				uid, _ := uuid.Parse(validUser)
				tokens := []db.FcmToken{
					{ID: uuid.New(), UserID: uid, Token: "tok-phone-A"},
					{ID: uuid.New(), UserID: uid, Token: "tok-tablet-B"},
				}
				d.repo.EXPECT().
					ListFCMTokensByUser(gomock.Any(), uid).
					Return(tokens, nil)

				// Verify the exact payload shape that hits FCM.
				d.fcm.EXPECT().
					Send(gomock.Any(),
						[]string{"tok-phone-A", "tok-tablet-B"},
						"Deck Clone Ready",
						gomock.Any(), // body string is asserted in DoAndReturn
						gomock.Any()).
					DoAndReturn(func(_ context.Context, _ []string, title, body string, data map[string]string) error {
						require.Contains(t, body, deckName)
						require.Equal(t, "deck_clone_completed", data["type"])
						require.Equal(t, deckID, data["deck_id"])
						return nil
					}).Times(1)

				// LogNotification gets called once with status="sent".
				d.repo.EXPECT().
					LogNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.LogNotificationParams) error {
						require.Equal(t, "sent", arg.Status)
						require.Equal(t, "fcm", arg.Channel)
						require.Equal(t, "deck_clone_ready", arg.NotificationType)
						return nil
					}).Times(1)
			},
		},
		{
			name:   "InternalError_FCMSendFails_LogsFailureAndPropagates",
			userID: validUser,
			setup: func(d deps) {
				uid, _ := uuid.Parse(validUser)
				d.repo.EXPECT().
					ListFCMTokensByUser(gomock.Any(), uid).
					Return([]db.FcmToken{{Token: "tok-x"}}, nil)

				d.fcm.EXPECT().
					Send(gomock.Any(), []string{"tok-x"}, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errFCM)

				// Log entry on the failure path must record status="failed".
				d.repo.EXPECT().
					LogNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, arg db.LogNotificationParams) error {
						require.Equal(t, "failed", arg.Status)
						require.NotNil(t, arg.ErrorMessage)
						return nil
					}).Times(1)
			},
			wantErr: errFCM,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			d := deps{
				repo: mock.NewMockNotificationRepository(ctrl),
				fcm:  mock.NewMockFCMSender(ctrl),
			}
			tc.setup(d)

			svc := New(d.repo, mock.NewMockMailer(ctrl), d.fcm, nil, Config{AppBaseURL: "https://mempan.app"})
			err := svc.SendDeckCloneReadyPush(context.Background(), tc.userID, deckID, deckName, cardCount)

			if tc.wantInvalidUUID {
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid user_id")
				return
			}
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
