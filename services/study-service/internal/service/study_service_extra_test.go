package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/mock/gomock"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/deckclient"
	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/mock"
)

// ---------------------------------------------------------------------------
// ReviewCard – success paths
// ---------------------------------------------------------------------------

func TestReviewCard_Success_DefaultWeights(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()
	sessionID := uuid.New()

	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(sessionID, cardID, userCardID) // ReviewedAt.Valid = false

	userCard := makeUserCard(userCardID, userID, cardID, deckID)
	updatedUC := userCard
	updatedUC.State = string(db.CardStateLearning)
	updatedUC.Reps = 1

	// No custom weights → use defaults.
	weightsRepo.EXPECT().GetActiveWeights(ctx, userID).Return(db.UserFsrsWeight{}, errors.New("no weights"))

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	scRepo.EXPECT().GetSessionCardByCard(ctx, db.GetSessionCardByCardParams{
		SessionID: sessionID,
		CardID:    cardID,
	}).Return(sc, nil)
	ucRepo.EXPECT().GetUserCardByID(ctx, userCardID).Return(userCard, nil)
	ucRepo.EXPECT().UpdateUserCardFSRS(ctx, gomock.Any()).Return(updatedUC, nil)
	revRepo.EXPECT().InsertRevlog(ctx, gomock.Any()).Return(db.Revlog{}, nil)
	scRepo.EXPECT().MarkSessionCardReviewed(ctx, db.MarkSessionCardReviewedParams{
		SessionID: sessionID,
		CardID:    cardID,
		Rating:    sql.NullInt32{Int32: 3, Valid: true},
	}).Return(db.SessionCard{}, nil)
	sessRepo.EXPECT().IncrementCompletedCards(ctx, sessionID).Return(db.StudySession{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID:  sessionID,
		UserID:     userID,
		CardID:     cardID,
		Rating:     3,
		DurationMS: 1500,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.UserCardID != updatedUC.UserCardID {
		t.Errorf("expected userCardID %v, got %v", updatedUC.UserCardID, result.UserCardID)
	}
}

func TestReviewCard_Success_WithCustomWeights(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()
	sessionID := uuid.New()

	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(sessionID, cardID, userCardID)
	userCard := makeUserCard(userCardID, userID, cardID, deckID)
	updatedUC := userCard

	customWeights := make(pq.Float64Array, 21)
	for i := range customWeights {
		customWeights[i] = float64(i+1) * 0.1
	}
	weightsRepo.EXPECT().GetActiveWeights(ctx, userID).Return(db.UserFsrsWeight{
		UserID:   userID,
		Weights:  customWeights,
		IsActive: true,
	}, nil)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	scRepo.EXPECT().GetSessionCardByCard(ctx, db.GetSessionCardByCardParams{
		SessionID: sessionID,
		CardID:    cardID,
	}).Return(sc, nil)
	ucRepo.EXPECT().GetUserCardByID(ctx, userCardID).Return(userCard, nil)
	ucRepo.EXPECT().UpdateUserCardFSRS(ctx, gomock.Any()).Return(updatedUC, nil)
	revRepo.EXPECT().InsertRevlog(ctx, gomock.Any()).Return(db.Revlog{}, nil)
	scRepo.EXPECT().MarkSessionCardReviewed(ctx, gomock.Any()).Return(db.SessionCard{}, nil)
	sessRepo.EXPECT().IncrementCompletedCards(ctx, sessionID).Return(db.StudySession{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID:  sessionID,
		UserID:     userID,
		CardID:     cardID,
		Rating:     4,
		DurationMS: 2000,
	})

	if err != nil {
		t.Fatalf("expected no error with custom weights, got %v", err)
	}
}

// ReviewCard with rating=1 (minimum valid rating) must not be rejected.
func TestReviewCard_RatingOne_Accepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()
	sessionID := uuid.New()

	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(sessionID, cardID, userCardID)
	userCard := makeUserCard(userCardID, userID, cardID, deckID)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	scRepo.EXPECT().GetSessionCardByCard(ctx, gomock.Any()).Return(sc, nil)
	ucRepo.EXPECT().GetUserCardByID(ctx, userCardID).Return(userCard, nil)
	weightsRepo.EXPECT().GetActiveWeights(ctx, userID).Return(db.UserFsrsWeight{}, errors.New("none"))
	ucRepo.EXPECT().UpdateUserCardFSRS(ctx, gomock.Any()).Return(userCard, nil)
	revRepo.EXPECT().InsertRevlog(ctx, gomock.Any()).Return(db.Revlog{}, nil)
	scRepo.EXPECT().MarkSessionCardReviewed(ctx, gomock.Any()).Return(db.SessionCard{}, nil)
	sessRepo.EXPECT().IncrementCompletedCards(ctx, sessionID).Return(db.StudySession{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID: sessionID,
		UserID:    userID,
		CardID:    cardID,
		Rating:    1, // minimum valid
	})

	if err != nil {
		t.Fatalf("rating 1 should be accepted, got %v", err)
	}
}

// ReviewCard with rating=0 must be rejected (boundary below valid range).
func TestReviewCard_RatingZero_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID: uuid.New(),
		UserID:    uuid.New(),
		CardID:    uuid.New(),
		Rating:    0,
	})

	if !errors.Is(err, domain.ErrInvalidRating) {
		t.Errorf("expected ErrInvalidRating for rating 0, got %v", err)
	}
}

// ElapsedDays is computed from LastReviewDate when it is set.
func TestReviewCard_ElapsedDaysFromLastReview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()
	sessionID := uuid.New()

	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(sessionID, cardID, userCardID)

	// Card was last reviewed 3 days ago.
	userCard := makeUserCard(userCardID, userID, cardID, deckID)
	userCard.LastReviewDate = sql.NullTime{
		Time:  time.Now().Add(-3 * 24 * time.Hour),
		Valid: true,
	}

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	scRepo.EXPECT().GetSessionCardByCard(ctx, gomock.Any()).Return(sc, nil)
	ucRepo.EXPECT().GetUserCardByID(ctx, userCardID).Return(userCard, nil)
	weightsRepo.EXPECT().GetActiveWeights(ctx, userID).Return(db.UserFsrsWeight{}, errors.New("none"))
	ucRepo.EXPECT().UpdateUserCardFSRS(ctx, gomock.Any()).Return(userCard, nil)
	revRepo.EXPECT().InsertRevlog(ctx, gomock.Any()).Return(db.Revlog{}, nil)
	scRepo.EXPECT().MarkSessionCardReviewed(ctx, gomock.Any()).Return(db.SessionCard{}, nil)
	sessRepo.EXPECT().IncrementCompletedCards(ctx, sessionID).Return(db.StudySession{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID: sessionID,
		UserID:    userID,
		CardID:    cardID,
		Rating:    2,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// StartSession – additional edge cases
// ---------------------------------------------------------------------------

// When due list is empty but new cards exist a session is still created.
func TestStartSession_OnlyNewCards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()
	sessionID := uuid.New()

	deckCards := []deckclient.CardInfo{{CardID: cardID, DeckID: deckID}}
	userCard := makeUserCard(userCardID, userID, cardID, deckID)
	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(sessionID, cardID, userCardID)

	sessRepo.EXPECT().GetOngoingSessionByDeck(ctx, gomock.Any()).Return(db.StudySession{}, domain.ErrSessionNotFound)
	deckClient.EXPECT().ListDeckCards(ctx, deckID, "tok").Return(deckCards, nil)
	ucRepo.EXPECT().UpsertUserCard(ctx, gomock.Any()).Return(userCard, nil)
	ucRepo.EXPECT().ListDueUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{}, nil) // no due cards
	ucRepo.EXPECT().ListNewUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{userCard}, nil)
	sessRepo.EXPECT().CreateStudySession(ctx, db.CreateStudySessionParams{
		UserID:     userID,
		DeckID:     deckID,
		TotalCards: 1,
	}).Return(session, nil)
	scRepo.EXPECT().InsertSessionCard(ctx, gomock.Any()).Return(sc, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.StartSession(ctx, StartSessionParams{
		UserID:      userID,
		DeckID:      deckID,
		AccessToken: "tok",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Cards) != 1 {
		t.Errorf("expected 1 card, got %d", len(result.Cards))
	}
}

// When both due and new card lists are empty the service returns ErrDeckEmpty.
func TestStartSession_NoDueOrNewCards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()

	deckCards := []deckclient.CardInfo{{CardID: cardID, DeckID: deckID}}
	userCard := makeUserCard(userCardID, userID, cardID, deckID)

	sessRepo.EXPECT().GetOngoingSessionByDeck(ctx, gomock.Any()).Return(db.StudySession{}, domain.ErrSessionNotFound)
	deckClient.EXPECT().ListDeckCards(ctx, deckID, "tok").Return(deckCards, nil)
	ucRepo.EXPECT().UpsertUserCard(ctx, gomock.Any()).Return(userCard, nil)
	ucRepo.EXPECT().ListDueUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{}, nil)
	ucRepo.EXPECT().ListNewUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.StartSession(ctx, StartSessionParams{
		UserID:      userID,
		DeckID:      deckID,
		AccessToken: "tok",
	})

	if !errors.Is(err, domain.ErrDeckEmpty) {
		t.Errorf("expected ErrDeckEmpty, got %v", err)
	}
}

// Zero limits are replaced with the package defaults (20 new, 200 review).
func TestStartSession_DefaultLimits(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()
	sessionID := uuid.New()

	deckCards := []deckclient.CardInfo{{CardID: cardID, DeckID: deckID}}
	userCard := makeUserCard(userCardID, userID, cardID, deckID)
	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(sessionID, cardID, userCardID)

	sessRepo.EXPECT().GetOngoingSessionByDeck(ctx, gomock.Any()).Return(db.StudySession{}, domain.ErrSessionNotFound)
	deckClient.EXPECT().ListDeckCards(ctx, deckID, "tok").Return(deckCards, nil)
	ucRepo.EXPECT().UpsertUserCard(ctx, gomock.Any()).Return(userCard, nil)

	// Expect default limits: review=200, new=20.
	ucRepo.EXPECT().ListDueUserCardsByDeck(ctx, db.ListDueUserCardsByDeckParams{
		UserID: userID,
		DeckID: deckID,
		Limit:  defaultReviewCardsLimit,
	}).Return([]db.UserCard{userCard}, nil)
	ucRepo.EXPECT().ListNewUserCardsByDeck(ctx, db.ListNewUserCardsByDeckParams{
		UserID: userID,
		DeckID: deckID,
		Limit:  defaultNewCardsLimit,
	}).Return([]db.UserCard{}, nil)
	sessRepo.EXPECT().CreateStudySession(ctx, gomock.Any()).Return(session, nil)
	scRepo.EXPECT().InsertSessionCard(ctx, gomock.Any()).Return(sc, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	// Pass zero limits to trigger the defaults.
	_, err := svc.StartSession(ctx, StartSessionParams{
		UserID:        userID,
		DeckID:        deckID,
		AccessToken:   "tok",
		NewCardsLimit: 0,
		ReviewLimit:   0,
	})

	if err != nil {
		t.Fatalf("expected no error with default limits, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRecentDecks – sorting
// ---------------------------------------------------------------------------

func TestGetRecentDecks_SortedByRecency(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckA := uuid.New()
	deckB := uuid.New()
	deckC := uuid.New()

	now := time.Now()
	rows := []db.ListRecentDecksRow{
		{DeckID: deckB, LastAccessedAt: now.Add(-2 * time.Hour)},
		{DeckID: deckC, LastAccessedAt: now.Add(-5 * time.Hour)},
		{DeckID: deckA, LastAccessedAt: now.Add(-1 * time.Minute)}, // most recent
	}

	sessRepo.EXPECT().ListRecentDecks(ctx, userID).Return(rows, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.GetRecentDecks(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 decks, got %d", len(result))
	}
	// Most recent first.
	if result[0].DeckID != deckA {
		t.Errorf("expected deckA first (most recent), got %v", result[0].DeckID)
	}
	if result[1].DeckID != deckB {
		t.Errorf("expected deckB second, got %v", result[1].DeckID)
	}
	if result[2].DeckID != deckC {
		t.Errorf("expected deckC last (oldest), got %v", result[2].DeckID)
	}
}

// ---------------------------------------------------------------------------
// GetDeckProgress – additional card states
// ---------------------------------------------------------------------------

// Relearning cards must be counted under LearnCount (same bucket as learning).
func TestGetDeckProgress_RelearningCards(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()

	cards := []db.UserCard{
		{UserCardID: uuid.New(), UserID: userID, CardID: uuid.New(), DeckID: deckID, State: string(db.CardStateRelearning)},
		{UserCardID: uuid.New(), UserID: userID, CardID: uuid.New(), DeckID: deckID, State: string(db.CardStateLearning)},
		{UserCardID: uuid.New(), UserID: userID, CardID: uuid.New(), DeckID: deckID, State: string(db.CardStateReview)},
	}

	ucRepo.EXPECT().ListUserCardsByDeck(ctx, db.ListUserCardsByDeckParams{
		UserID: userID,
		DeckID: deckID,
	}).Return(cards, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	progress, err := svc.GetDeckProgress(ctx, userID, deckID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if progress.LearnCount != 2 {
		t.Errorf("expected LearnCount 2 (learning + relearning), got %d", progress.LearnCount)
	}
	if progress.MemorizedCount != 1 {
		t.Errorf("expected MemorizedCount 1, got %d", progress.MemorizedCount)
	}
	if progress.NewCount != 0 {
		t.Errorf("expected NewCount 0, got %d", progress.NewCount)
	}
	if progress.TotalCount != 3 {
		t.Errorf("expected TotalCount 3, got %d", progress.TotalCount)
	}
}

// An empty deck returns zero counts across the board.
func TestGetDeckProgress_EmptyDeck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()

	ucRepo.EXPECT().ListUserCardsByDeck(ctx, db.ListUserCardsByDeckParams{
		UserID: userID,
		DeckID: deckID,
	}).Return([]db.UserCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	progress, err := svc.GetDeckProgress(ctx, userID, deckID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if progress.TotalCount != 0 || progress.NewCount != 0 || progress.LearnCount != 0 || progress.MemorizedCount != 0 {
		t.Errorf("expected all zero counts for empty deck, got %+v", progress)
	}
	if len(progress.Tags) != 3 {
		t.Errorf("expected 3 tags regardless of count, got %d", len(progress.Tags))
	}
}

// GetDeckProgress tags should carry the correct card ID lists.
func TestGetDeckProgress_TagCardIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	userID := uuid.New()
	deckID := uuid.New()
	newCardID := uuid.New()
	reviewCardID := uuid.New()

	cards := []db.UserCard{
		{UserCardID: uuid.New(), UserID: userID, CardID: newCardID, DeckID: deckID, State: string(db.CardStateNew)},
		{UserCardID: uuid.New(), UserID: userID, CardID: reviewCardID, DeckID: deckID, State: string(db.CardStateReview)},
	}

	ucRepo.EXPECT().ListUserCardsByDeck(ctx, gomock.Any()).Return(cards, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	progress, err := svc.GetDeckProgress(ctx, userID, deckID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var newTag, memorizedTag *ProgressTag
	for i := range progress.Tags {
		switch progress.Tags[i].Label {
		case "new":
			newTag = &progress.Tags[i]
		case "memorized":
			memorizedTag = &progress.Tags[i]
		}
	}

	if newTag == nil || len(newTag.CardIDs) != 1 || newTag.CardIDs[0] != newCardID {
		t.Errorf("new tag card IDs incorrect: %+v", newTag)
	}
	if memorizedTag == nil || len(memorizedTag.CardIDs) != 1 || memorizedTag.CardIDs[0] != reviewCardID {
		t.Errorf("memorized tag card IDs incorrect: %+v", memorizedTag)
	}
}
