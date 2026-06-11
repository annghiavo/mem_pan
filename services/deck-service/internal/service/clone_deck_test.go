package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/mock"
	"mem_pan/services/deck-service/internal/publisher"
)

// TestDeckService_CloneDeck covers the three required scenarios per the test
// strategy: Success / Bad Input / Internal Error. Side-effects on the
// EventPublisher are explicitly verified via Times(N).
func TestDeckService_CloneDeck(t *testing.T) {
	t.Parallel()

	var (
		sourceDeckID = uuid.New()
		sourceOwner  = uuid.New()
		newOwner     = uuid.New()
		errDB        = errors.New("connection refused")
	)

	mkSource := func(public bool, status db.ContentStatus) db.Deck {
		return db.Deck{
			DeckID:    sourceDeckID,
			UserID:    sourceOwner,
			Name:      "Deutsch A1",
			IsPublic:  public,
			Status:    string(status),
			Settings:  []byte(`{"quiz_type":"multiple_choice"}`),
			CardCount: 2,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}

	mkClonedDeck := func() db.Deck {
		return db.Deck{
			DeckID:    uuid.New(),
			UserID:    newOwner,
			Name:      "Copy of Deutsch A1",
			IsPublic:  false,
			Status:    string(db.ContentStatusActive),
			Settings:  []byte(`{"quiz_type":"multiple_choice"}`),
			CardCount: 2,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}

	mkClonedCards := func(deckID uuid.UUID) []db.ListCardsByDeckRow {
		return []db.ListCardsByDeckRow{
			{
				CardID:       uuid.New(),
				UserID:       newOwner,
				DeckID:       deckID,
				NoteID:       uuid.New(),
				Position:     0,
				CreatedAt:    time.Now().UTC(),
				ContentFront: "Hallo",
				ContentBack:  "Hello",
				ImageUrl:     sql.NullString{},
				LangFront:    "de",
				LangBack:     "en",
			},
			{
				CardID:       uuid.New(),
				UserID:       newOwner,
				DeckID:       deckID,
				NoteID:       uuid.New(),
				Position:     1,
				CreatedAt:    time.Now().UTC(),
				ContentFront: "Danke",
				ContentBack:  "Thanks",
				ImageUrl:     sql.NullString{String: "https://cdn/x.png", Valid: true},
				LangFront:    "de",
				LangBack:     "en",
			},
		}
	}

	type deps struct {
		deckRepo *mock.MockDeckRepository
		cardRepo *mock.MockCardRepository
		pub      *mock.MockEventPublisher
	}

	tests := []struct {
		name          string
		sourceID      uuid.UUID
		newOwner      uuid.UUID
		setup         func(d deps)
		wantErr       error
		wantNewOwner  uuid.UUID
		wantNewName   string
		assertNonOwn  bool
	}{
		{
			name:         "Success_PublicDeck_EmitsDeckAndCardEvents",
			sourceID:     sourceDeckID,
			newOwner:     newOwner,
			wantNewOwner: newOwner,
			wantNewName:  "Copy of Deutsch A1",
			setup: func(d deps) {
				src := mkSource(true, db.ContentStatusActive)
				cloned := mkClonedDeck()
				cards := mkClonedCards(cloned.DeckID)

				d.deckRepo.EXPECT().
					GetDeckByID(gomock.Any(), sourceDeckID).
					Return(src, nil)
				d.deckRepo.EXPECT().
					CloneDeck(gomock.Any(), src, newOwner, "Copy of Deutsch A1").
					Return(cloned, cards, nil)

				// 1 deck.created + 2 card.created.
				d.pub.EXPECT().
					PublishDeckCreated(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
				d.pub.EXPECT().
					PublishCardCreated(gomock.Any(), gomock.Any()).
					Times(len(cards)).
					Return(nil)
			},
		},
		{
			name:         "Success_OwnerClonesOwnPrivateDeck",
			sourceID:     sourceDeckID,
			newOwner:     sourceOwner,
			wantNewOwner: sourceOwner,
			wantNewName:  "Copy of Deutsch A1",
			setup: func(d deps) {
				src := mkSource(false, db.ContentStatusActive)
				cloned := mkClonedDeck()
				cloned.UserID = sourceOwner
				cards := mkClonedCards(cloned.DeckID)

				d.deckRepo.EXPECT().
					GetDeckByID(gomock.Any(), sourceDeckID).
					Return(src, nil)
				d.deckRepo.EXPECT().
					CloneDeck(gomock.Any(), src, sourceOwner, "Copy of Deutsch A1").
					Return(cloned, cards, nil)
				d.pub.EXPECT().PublishDeckCreated(gomock.Any(), gomock.Any()).Times(1).Return(nil)
				d.pub.EXPECT().PublishCardCreated(gomock.Any(), gomock.Any()).Times(len(cards)).Return(nil)
			},
		},
		{
			name:     "BadInput_SourceDeleted_ReturnsDeckNotFound",
			sourceID: sourceDeckID,
			newOwner: newOwner,
			setup: func(d deps) {
				deleted := mkSource(true, db.ContentStatusDeleted)
				d.deckRepo.EXPECT().
					GetDeckByID(gomock.Any(), sourceDeckID).
					Return(deleted, nil)
				// No CloneDeck, no publish.
				d.deckRepo.EXPECT().CloneDeck(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				d.pub.EXPECT().PublishDeckCreated(gomock.Any(), gomock.Any()).Times(0)
				d.pub.EXPECT().PublishCardCreated(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: domain.ErrDeckNotFound,
		},
		{
			name:     "BadInput_PrivateDeckOtherOwner_ReturnsForbidden",
			sourceID: sourceDeckID,
			newOwner: newOwner,
			setup: func(d deps) {
				private := mkSource(false, db.ContentStatusActive)
				d.deckRepo.EXPECT().
					GetDeckByID(gomock.Any(), sourceDeckID).
					Return(private, nil)
				d.deckRepo.EXPECT().CloneDeck(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				d.pub.EXPECT().PublishDeckCreated(gomock.Any(), gomock.Any()).Times(0)
				d.pub.EXPECT().PublishCardCreated(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: domain.ErrForbidden,
		},
		{
			name:     "InternalError_GetDeckByID_DBDown",
			sourceID: sourceDeckID,
			newOwner: newOwner,
			setup: func(d deps) {
				d.deckRepo.EXPECT().
					GetDeckByID(gomock.Any(), sourceDeckID).
					Return(db.Deck{}, errDB)
				d.deckRepo.EXPECT().CloneDeck(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				d.pub.EXPECT().PublishDeckCreated(gomock.Any(), gomock.Any()).Times(0)
				d.pub.EXPECT().PublishCardCreated(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errDB,
		},
		{
			name:     "InternalError_CloneDeck_TxFails_NoEvents",
			sourceID: sourceDeckID,
			newOwner: newOwner,
			setup: func(d deps) {
				src := mkSource(true, db.ContentStatusActive)
				d.deckRepo.EXPECT().
					GetDeckByID(gomock.Any(), sourceDeckID).
					Return(src, nil)
				d.deckRepo.EXPECT().
					CloneDeck(gomock.Any(), src, newOwner, "Copy of Deutsch A1").
					Return(db.Deck{}, nil, errDB)

				// Important contract: if clone tx fails, NO event must escape.
				d.pub.EXPECT().PublishDeckCreated(gomock.Any(), gomock.Any()).Times(0)
				d.pub.EXPECT().PublishCardCreated(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: errDB,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			d := deps{
				deckRepo: mock.NewMockDeckRepository(ctrl),
				cardRepo: mock.NewMockCardRepository(ctrl),
				pub:      mock.NewMockEventPublisher(ctrl),
			}
			tc.setup(d)

			svc := NewDeckService(d.deckRepo, d.cardRepo, d.pub)
			got, err := svc.CloneDeck(context.Background(), tc.sourceID, tc.newOwner, false)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, uuid.Nil, got.DeckID, "no deck should be returned on error")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantNewOwner, got.UserID)
			require.Equal(t, tc.wantNewName, got.Name)
			require.Equal(t, string(db.ContentStatusActive), got.Status)
		})
	}
}

// Compile-time assertion: MockEventPublisher satisfies publisher.EventPublisher.
var _ publisher.EventPublisher = (*mock.MockEventPublisher)(nil)
