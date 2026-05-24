package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/deckclient"
	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/mock"
)

func makeSession(sessionID, userID, deckID uuid.UUID, status db.SessionStatus) db.StudySession {
	return db.StudySession{
		SessionID:      sessionID,
		UserID:         userID,
		DeckID:         deckID,
		Status:         string(status),
		TotalCards:     1,
		CompletedCards: 0,
		StartedAt:      time.Now(),
		LastAccessedAt: time.Now(),
	}
}

func makeUserCard(userCardID, userID, cardID, deckID uuid.UUID) db.UserCard {
	return db.UserCard{
		UserCardID:     userCardID,
		UserID:         userID,
		CardID:         cardID,
		DeckID:         deckID,
		State:          string(db.CardStateNew),
		NextReviewDate: time.Now().Add(-time.Hour),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func makeSessionCard(sessionID, cardID, userCardID uuid.UUID) db.SessionCard {
	return db.SessionCard{
		SessionID:  sessionID,
		Position:   0,
		CardID:     cardID,
		UserCardID: userCardID,
	}
}

func newTestStudyService(
	ctrl *gomock.Controller,
	ucRepo *mock.MockUserCardRepository,
	sessRepo *mock.MockStudySessionRepository,
	scRepo *mock.MockSessionCardRepository,
	revRepo *mock.MockRevlogRepository,
	weightsRepo *mock.MockFsrsWeightsRepository,
	deckClient *mock.MockDeckClient,
) StudyService {
	return NewStudyService(ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
}

// ------- StartSession (abandons any ongoing session, then creates a new one) -------

func TestStartSession_AbandonsOngoingAndCreatesNew(t *testing.T) {
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
	oldSessionID := uuid.New()
	newSessionID := uuid.New()
	cardID := uuid.New()
	userCardID := uuid.New()

	oldSession := makeSession(oldSessionID, userID, deckID, db.SessionStatusOngoing)
	deckCards := []deckclient.CardInfo{{CardID: cardID, DeckID: deckID}}
	userCard := makeUserCard(userCardID, userID, cardID, deckID)
	newSession := makeSession(newSessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(newSessionID, cardID, userCardID)

	sessRepo.EXPECT().GetOngoingSessionByDeck(ctx, db.GetOngoingSessionByDeckParams{
		UserID: userID,
		DeckID: deckID,
	}).Return(oldSession, nil)
	sessRepo.EXPECT().AbandonStudySession(ctx, oldSessionID).Return(oldSession, nil)
	deckClient.EXPECT().ListDeckCards(ctx, deckID, "token123").Return(deckCards, nil)
	ucRepo.EXPECT().UpsertUserCard(ctx, db.UpsertUserCardParams{
		UserID: userID,
		CardID: cardID,
		DeckID: deckID,
	}).Return(userCard, nil)
	ucRepo.EXPECT().ListDueUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{userCard}, nil)
	ucRepo.EXPECT().ListNewUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{}, nil)
	sessRepo.EXPECT().CreateStudySession(ctx, db.CreateStudySessionParams{
		UserID:     userID,
		DeckID:     deckID,
		TotalCards: 1,
	}).Return(newSession, nil)
	scRepo.EXPECT().InsertSessionCard(ctx, gomock.Any()).Return(sc, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.StartSession(ctx, StartSessionParams{
		UserID:      userID,
		DeckID:      deckID,
		AccessToken: "token123",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Session.SessionID != newSessionID {
		t.Errorf("expected new sessionID %v, got %v", newSessionID, result.Session.SessionID)
	}
}

// ------- StartSession (new session) -------

func TestStartSession_NewSession(t *testing.T) {
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
	sessionID := uuid.New()
	userCardID := uuid.New()

	deckCards := []deckclient.CardInfo{{CardID: cardID, DeckID: deckID}}
	userCard := makeUserCard(userCardID, userID, cardID, deckID)
	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	sc := makeSessionCard(sessionID, cardID, userCardID)

	sessRepo.EXPECT().GetOngoingSessionByDeck(ctx, gomock.Any()).Return(db.StudySession{}, domain.ErrSessionNotFound)
	deckClient.EXPECT().ListDeckCards(ctx, deckID, "token123").Return(deckCards, nil)
	ucRepo.EXPECT().UpsertUserCard(ctx, db.UpsertUserCardParams{
		UserID: userID,
		CardID: cardID,
		DeckID: deckID,
	}).Return(userCard, nil)
	ucRepo.EXPECT().ListDueUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{userCard}, nil)
	ucRepo.EXPECT().ListNewUserCardsByDeck(ctx, gomock.Any()).Return([]db.UserCard{}, nil)
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
		AccessToken: "token123",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Session.SessionID != sessionID {
		t.Errorf("expected sessionID %v, got %v", sessionID, result.Session.SessionID)
	}
}

func TestStartSession_EmptyDeck(t *testing.T) {
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

	sessRepo.EXPECT().GetOngoingSessionByDeck(ctx, gomock.Any()).Return(db.StudySession{}, domain.ErrSessionNotFound)
	deckClient.EXPECT().ListDeckCards(ctx, deckID, "token").Return([]deckclient.CardInfo{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.StartSession(ctx, StartSessionParams{
		UserID:      userID,
		DeckID:      deckID,
		AccessToken: "token",
	})

	if !errors.Is(err, domain.ErrDeckEmpty) {
		t.Errorf("expected ErrDeckEmpty, got %v", err)
	}
}

// ------- GetSession -------

func TestGetSession_Success(t *testing.T) {
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
	sessionID := uuid.New()

	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	scRepo.EXPECT().ListSessionCards(ctx, sessionID).Return([]db.SessionCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.GetSession(ctx, sessionID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Session.SessionID != sessionID {
		t.Errorf("expected sessionID %v, got %v", sessionID, result.Session.SessionID)
	}
}

func TestGetSession_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	sessionID := uuid.New()

	session := makeSession(sessionID, ownerID, deckID, db.SessionStatusOngoing)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.GetSession(ctx, sessionID, otherID)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- ReviewCard -------

func TestReviewCard_InvalidRating(t *testing.T) {
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
		Rating:    5, // invalid
	})

	if !errors.Is(err, domain.ErrInvalidRating) {
		t.Errorf("expected ErrInvalidRating, got %v", err)
	}
}

func TestReviewCard_SessionForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	sessionID := uuid.New()
	session := makeSession(sessionID, ownerID, deckID, db.SessionStatusOngoing)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID: sessionID,
		UserID:    otherID,
		CardID:    uuid.New(),
		Rating:    3,
	})

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestReviewCard_AlreadyFinished(t *testing.T) {
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
	sessionID := uuid.New()
	session := makeSession(sessionID, userID, deckID, db.SessionStatusCompleted)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID: sessionID,
		UserID:    userID,
		CardID:    uuid.New(),
		Rating:    3,
	})

	if !errors.Is(err, domain.ErrSessionFinished) {
		t.Errorf("expected ErrSessionFinished, got %v", err)
	}
}

func TestReviewCard_CardAlreadyReviewed(t *testing.T) {
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
	sc.ReviewedAt.Valid = true
	sc.ReviewedAt.Time = time.Now()

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	scRepo.EXPECT().GetSessionCardByCard(ctx, db.GetSessionCardByCardParams{
		SessionID: sessionID,
		CardID:    cardID,
	}).Return(sc, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.ReviewCard(ctx, ReviewCardParams{
		SessionID: sessionID,
		UserID:    userID,
		CardID:    cardID,
		Rating:    3,
	})

	if !errors.Is(err, domain.ErrCardAlreadyReviewed) {
		t.Errorf("expected ErrCardAlreadyReviewed, got %v", err)
	}
}

// ------- FinishSession -------

func TestFinishSession_Success(t *testing.T) {
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
	sessionID := uuid.New()
	session := makeSession(sessionID, userID, deckID, db.SessionStatusOngoing)
	finished := makeSession(sessionID, userID, deckID, db.SessionStatusCompleted)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	sessRepo.EXPECT().FinishStudySession(ctx, sessionID).Return(finished, nil)
	scRepo.EXPECT().ListSessionCards(ctx, sessionID).Return([]db.SessionCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.FinishSession(ctx, sessionID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Session.Status != string(db.SessionStatusCompleted) {
		t.Errorf("expected completed status, got %s", result.Session.Status)
	}
}

func TestFinishSession_AlreadyFinished(t *testing.T) {
	// /finish is idempotent — calling it on a session that auto-completed
	// (when the last review hit total_cards) must return the existing session
	// rather than erroring, so the client's explicit follow-up call is safe.
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
	sessionID := uuid.New()
	session := makeSession(sessionID, userID, deckID, db.SessionStatusCompleted)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)
	scRepo.EXPECT().ListSessionCards(ctx, sessionID).Return([]db.SessionCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.FinishSession(ctx, sessionID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.Session.SessionID != sessionID {
		t.Errorf("expected existing session returned, got %+v", result)
	}
}

func TestFinishSession_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ucRepo := mock.NewMockUserCardRepository(ctrl)
	sessRepo := mock.NewMockStudySessionRepository(ctrl)
	scRepo := mock.NewMockSessionCardRepository(ctrl)
	revRepo := mock.NewMockRevlogRepository(ctrl)
	weightsRepo := mock.NewMockFsrsWeightsRepository(ctrl)
	deckClient := mock.NewMockDeckClient(ctrl)

	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	deckID := uuid.New()
	sessionID := uuid.New()
	session := makeSession(sessionID, ownerID, deckID, db.SessionStatusOngoing)

	sessRepo.EXPECT().GetStudySession(ctx, sessionID).Return(session, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.FinishSession(ctx, sessionID, otherID)

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// ------- GetDueCards -------

func TestGetDueCards_AllDecks(t *testing.T) {
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

	ucRepo.EXPECT().ListDueUserCards(ctx, db.ListDueUserCardsParams{UserID: userID, Limit: 1000}).Return([]db.UserCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	cards, err := svc.GetDueCards(ctx, userID, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cards == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestGetDueCards_SpecificDeck(t *testing.T) {
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

	ucRepo.EXPECT().ListDueUserCardsByDeck(ctx, db.ListDueUserCardsByDeckParams{
		UserID: userID,
		DeckID: deckID,
		Limit:  1000,
	}).Return([]db.UserCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	cards, err := svc.GetDueCards(ctx, userID, &deckID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cards == nil {
		t.Error("expected empty slice, got nil")
	}
}

// ------- GetRecentSessionCards -------

func TestGetRecentSessionCards_Success(t *testing.T) {
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
	sessionID := uuid.New()
	session := makeSession(sessionID, userID, deckID, db.SessionStatusCompleted)

	sessRepo.EXPECT().GetMostRecentSession(ctx, userID).Return(session, nil)
	scRepo.EXPECT().ListSessionCards(ctx, sessionID).Return([]db.SessionCard{}, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.GetRecentSessionCards(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Session.SessionID != sessionID {
		t.Errorf("expected sessionID %v, got %v", sessionID, result.Session.SessionID)
	}
}

func TestGetRecentSessionCards_NoSession(t *testing.T) {
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

	sessRepo.EXPECT().GetMostRecentSession(ctx, userID).Return(db.StudySession{}, domain.ErrSessionNotFound)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	_, err := svc.GetRecentSessionCards(ctx, userID)

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// ------- GetRecentDecks -------

func TestGetRecentDecks_Success(t *testing.T) {
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
	rows := []db.ListRecentDecksRow{
		{DeckID: deckID, LastAccessedAt: time.Now()},
	}

	sessRepo.EXPECT().ListRecentDecks(ctx, userID).Return(rows, nil)

	svc := newTestStudyService(ctrl, ucRepo, sessRepo, scRepo, revRepo, weightsRepo, deckClient)
	result, err := svc.GetRecentDecks(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 recent deck, got %d", len(result))
	}
	if result[0].DeckID != deckID {
		t.Errorf("expected deckID %v, got %v", deckID, result[0].DeckID)
	}
}

// ------- GetDeckProgress -------

func TestGetDeckProgress_Success(t *testing.T) {
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
	cardID1 := uuid.New()
	cardID2 := uuid.New()
	cardID3 := uuid.New()

	cards := []db.UserCard{
		{UserCardID: uuid.New(), UserID: userID, CardID: cardID1, DeckID: deckID, State: string(db.CardStateNew)},
		{UserCardID: uuid.New(), UserID: userID, CardID: cardID2, DeckID: deckID, State: string(db.CardStateLearning)},
		{UserCardID: uuid.New(), UserID: userID, CardID: cardID3, DeckID: deckID, State: string(db.CardStateReview)},
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
	if progress.TotalCount != 3 {
		t.Errorf("expected total 3, got %d", progress.TotalCount)
	}
	if progress.NewCount != 1 {
		t.Errorf("expected new 1, got %d", progress.NewCount)
	}
	if progress.LearnCount != 1 {
		t.Errorf("expected learn 1, got %d", progress.LearnCount)
	}
	if progress.MemorizedCount != 1 {
		t.Errorf("expected memorized 1, got %d", progress.MemorizedCount)
	}
}
