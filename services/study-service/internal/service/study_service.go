package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/deckclient"
	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/fsrs"
	"mem_pan/services/study-service/internal/repository"
)

const (
	defaultNewCardsLimit    = int32(20)
	defaultReviewCardsLimit = int32(200)
)

type StartSessionParams struct {
	UserID        uuid.UUID
	DeckID        uuid.UUID
	NewCardsLimit int32
	ReviewLimit   int32
	AccessToken   string
}

type ReviewCardParams struct {
	SessionID  uuid.UUID
	UserID     uuid.UUID
	CardID     uuid.UUID
	Rating     int32
	DurationMS int32
}

type SessionResult struct {
	Session db.StudySession
	Cards   []db.SessionCard
}

type RecentDeck struct {
	DeckID         uuid.UUID
	LastAccessedAt time.Time
}

type DeckProgress struct {
	DeckID         uuid.UUID
	NewCount       int32
	StudyingCount  int32
	MemorizedCount int32
	TotalCount     int32
}

type StudyService interface {
	StartSession(ctx context.Context, p StartSessionParams) (*SessionResult, error)
	GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*SessionResult, error)
	ReviewCard(ctx context.Context, p ReviewCardParams) (db.UserCard, error)
	FinishSession(ctx context.Context, sessionID, userID uuid.UUID) (*SessionResult, error)
	GetDueCards(ctx context.Context, userID uuid.UUID, deckID *uuid.UUID) ([]db.UserCard, error)
	GetRecentSessionCards(ctx context.Context, userID uuid.UUID) (*SessionResult, error)
	GetRecentDecks(ctx context.Context, userID uuid.UUID) ([]RecentDeck, error)
	GetDeckProgress(ctx context.Context, userID, deckID uuid.UUID) (*DeckProgress, error)
}

type studyService struct {
	userCardRepo    repository.UserCardRepository
	sessionRepo     repository.StudySessionRepository
	sessionCardRepo repository.SessionCardRepository
	revlogRepo      repository.RevlogRepository
	weightsRepo     repository.FsrsWeightsRepository
	deckClient      deckclient.Client
}

func NewStudyService(
	userCardRepo repository.UserCardRepository,
	sessionRepo repository.StudySessionRepository,
	sessionCardRepo repository.SessionCardRepository,
	revlogRepo repository.RevlogRepository,
	weightsRepo repository.FsrsWeightsRepository,
	deckClient deckclient.Client,
) StudyService {
	return &studyService{
		userCardRepo:    userCardRepo,
		sessionRepo:     sessionRepo,
		sessionCardRepo: sessionCardRepo,
		revlogRepo:      revlogRepo,
		weightsRepo:     weightsRepo,
		deckClient:      deckClient,
	}
}

func (s *studyService) StartSession(ctx context.Context, p StartSessionParams) (*SessionResult, error) {
	// Resume existing ongoing session.
	existing, err := s.sessionRepo.GetOngoingSessionByDeck(ctx, db.GetOngoingSessionByDeckParams{
		UserID: p.UserID,
		DeckID: p.DeckID,
	})
	if err == nil {
		cards, err := s.sessionCardRepo.ListSessionCards(ctx, existing.SessionID)
		if err != nil {
			return nil, err
		}
		return &SessionResult{Session: existing, Cards: cards}, nil
	}
	if !errors.Is(err, domain.ErrSessionNotFound) {
		return nil, err
	}

	// Fetch all cards in the deck from deck-service.
	deckCards, err := s.deckClient.ListDeckCards(ctx, p.DeckID, p.AccessToken)
	if err != nil {
		return nil, err
	}
	if len(deckCards) == 0 {
		return nil, domain.ErrDeckEmpty
	}

	// Upsert user_card records for every card in the deck.
	for _, dc := range deckCards {
		_, err := s.userCardRepo.UpsertUserCard(ctx, db.UpsertUserCardParams{
			UserID: p.UserID,
			CardID: dc.CardID,
			DeckID: dc.DeckID,
		})
		if err != nil {
			return nil, err
		}
	}

	newLimit := p.NewCardsLimit
	if newLimit <= 0 {
		newLimit = defaultNewCardsLimit
	}
	reviewLimit := p.ReviewLimit
	if reviewLimit <= 0 {
		reviewLimit = defaultReviewCardsLimit
	}

	// Select due review cards (not new).
	dueCards, err := s.userCardRepo.ListDueUserCardsByDeck(ctx, db.ListDueUserCardsByDeckParams{
		UserID: p.UserID,
		DeckID: p.DeckID,
		Limit:  reviewLimit,
	})
	if err != nil {
		return nil, err
	}

	// Select new cards.
	newCards, err := s.userCardRepo.ListNewUserCardsByDeck(ctx, db.ListNewUserCardsByDeckParams{
		UserID: p.UserID,
		DeckID: p.DeckID,
		Limit:  newLimit,
	})
	if err != nil {
		return nil, err
	}

	selected := append(dueCards, newCards...)
	if len(selected) == 0 {
		return nil, domain.ErrDeckEmpty
	}

	session, err := s.sessionRepo.CreateStudySession(ctx, db.CreateStudySessionParams{
		UserID:     p.UserID,
		DeckID:     p.DeckID,
		TotalCards: int32(len(selected)),
	})
	if err != nil {
		return nil, err
	}

	sessionCards := make([]db.SessionCard, 0, len(selected))
	for i, uc := range selected {
		sc, err := s.sessionCardRepo.InsertSessionCard(ctx, db.InsertSessionCardParams{
			SessionID:  session.SessionID,
			Position:   int32(i),
			CardID:     uc.CardID,
			UserCardID: uc.UserCardID,
		})
		if err != nil {
			return nil, err
		}
		sessionCards = append(sessionCards, sc)
	}

	return &SessionResult{Session: session, Cards: sessionCards}, nil
}

func (s *studyService) GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*SessionResult, error) {
	session, err := s.sessionRepo.GetStudySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, domain.ErrForbidden
	}

	cards, err := s.sessionCardRepo.ListSessionCards(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return &SessionResult{Session: session, Cards: cards}, nil
}

func (s *studyService) ReviewCard(ctx context.Context, p ReviewCardParams) (db.UserCard, error) {
	if p.Rating < 1 || p.Rating > 4 {
		return db.UserCard{}, domain.ErrInvalidRating
	}

	session, err := s.sessionRepo.GetStudySession(ctx, p.SessionID)
	if err != nil {
		return db.UserCard{}, err
	}
	if session.UserID != p.UserID {
		return db.UserCard{}, domain.ErrForbidden
	}
	if session.Status != string(db.SessionStatusOngoing) {
		return db.UserCard{}, domain.ErrSessionFinished
	}

	sc, err := s.sessionCardRepo.GetSessionCardByCard(ctx, db.GetSessionCardByCardParams{
		SessionID: p.SessionID,
		CardID:    p.CardID,
	})
	if err != nil {
		return db.UserCard{}, err
	}
	if sc.ReviewedAt.Valid {
		return db.UserCard{}, domain.ErrCardAlreadyReviewed
	}

	uc, err := s.userCardRepo.GetUserCardByID(ctx, sc.UserCardID)
	if err != nil {
		return db.UserCard{}, err
	}

	// Load user FSRS weights (use defaults if none saved).
	params := fsrs.DefaultParams()
	weights, err := s.weightsRepo.GetActiveWeights(ctx, p.UserID)
	if err == nil && len(weights.Weights) == 21 {
		params = fsrs.ParamsFromWeights([]float64(weights.Weights))
	}

	now := time.Now()
	fsrsCard := fsrs.UserCardToFSRS(uc)
	result := fsrs.Schedule(params, fsrsCard, gofsrs.Rating(p.Rating), now)
	newCard := result.Card

	// Compute elapsed days since last review.
	elapsedDays := int32(0)
	if uc.LastReviewDate.Valid {
		elapsedDays = int32(now.Sub(uc.LastReviewDate.Time).Hours() / 24)
	}

	updatedUC, err := s.userCardRepo.UpdateUserCardFSRS(ctx, db.UpdateUserCardFSRSParams{
		UserCardID:     uc.UserCardID,
		State:          fsrs.FSRSStateToString(newCard.State),
		Stability:      newCard.Stability,
		Difficulty:     newCard.Difficulty,
		Reps:           int32(newCard.Reps),
		Lapses:         int32(newCard.Lapses),
		ScheduledDays:  int32(newCard.ScheduledDays),
		NextReviewDate: newCard.Due,
		LastReviewDate: sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		return db.UserCard{}, err
	}

	_, err = s.revlogRepo.InsertRevlog(ctx, db.InsertRevlogParams{
		UserID:           p.UserID,
		CardID:           uc.CardID,
		UserCardID:       uc.UserCardID,
		SessionID:        uuid.NullUUID{UUID: p.SessionID, Valid: true},
		Rating:           int16(p.Rating),
		DurationMs:       p.DurationMS,
		StateBefore:      uc.State,
		StabilityBefore:  uc.Stability,
		DifficultyBefore: uc.Difficulty,
		ElapsedDays:      elapsedDays,
		ScheduledDays:    uc.ScheduledDays,
		StateAfter:       updatedUC.State,
		StabilityAfter:   updatedUC.Stability,
		DifficultyAfter:  updatedUC.Difficulty,
	})
	if err != nil {
		return db.UserCard{}, err
	}

	_, err = s.sessionCardRepo.MarkSessionCardReviewed(ctx, db.MarkSessionCardReviewedParams{
		SessionID: p.SessionID,
		CardID:    p.CardID,
		Rating:    sql.NullInt32{Int32: p.Rating, Valid: true},
	})
	if err != nil {
		return db.UserCard{}, err
	}

	_, err = s.sessionRepo.IncrementCompletedCards(ctx, p.SessionID)
	if err != nil {
		return db.UserCard{}, err
	}

	return updatedUC, nil
}

func (s *studyService) FinishSession(ctx context.Context, sessionID, userID uuid.UUID) (*SessionResult, error) {
	session, err := s.sessionRepo.GetStudySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, domain.ErrForbidden
	}
	if session.Status != string(db.SessionStatusOngoing) {
		return nil, domain.ErrSessionFinished
	}

	finished, err := s.sessionRepo.FinishStudySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	cards, err := s.sessionCardRepo.ListSessionCards(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return &SessionResult{Session: finished, Cards: cards}, nil
}

func (s *studyService) GetDueCards(ctx context.Context, userID uuid.UUID, deckID *uuid.UUID) ([]db.UserCard, error) {
	if deckID != nil {
		return s.userCardRepo.ListDueUserCardsByDeck(ctx, db.ListDueUserCardsByDeckParams{
			UserID: userID,
			DeckID: *deckID,
			Limit:  1000,
		})
	}
	return s.userCardRepo.ListDueUserCards(ctx, db.ListDueUserCardsParams{
		UserID: userID,
		Limit:  1000,
	})
}

func (s *studyService) GetRecentSessionCards(ctx context.Context, userID uuid.UUID) (*SessionResult, error) {
	session, err := s.sessionRepo.GetMostRecentSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	cards, err := s.sessionCardRepo.ListSessionCards(ctx, session.SessionID)
	if err != nil {
		return nil, err
	}
	return &SessionResult{Session: session, Cards: cards}, nil
}

func (s *studyService) GetRecentDecks(ctx context.Context, userID uuid.UUID) ([]RecentDeck, error) {
	rows, err := s.sessionRepo.ListRecentDecks(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Sort by most recent first (DISTINCT ON orders by deck_id).
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].LastAccessedAt.After(rows[j].LastAccessedAt)
	})
	result := make([]RecentDeck, len(rows))
	for i, r := range rows {
		result[i] = RecentDeck{
			DeckID:         r.DeckID,
			LastAccessedAt: r.LastAccessedAt,
		}
	}
	return result, nil
}

func (s *studyService) GetDeckProgress(ctx context.Context, userID, deckID uuid.UUID) (*DeckProgress, error) {
	cards, err := s.userCardRepo.ListUserCardsByDeck(ctx, db.ListUserCardsByDeckParams{
		UserID: userID,
		DeckID: deckID,
	})
	if err != nil {
		return nil, err
	}
	progress := &DeckProgress{DeckID: deckID}
	for _, c := range cards {
		switch c.State {
		case string(db.CardStateNew):
			progress.NewCount++
		case string(db.CardStateLearning), string(db.CardStateRelearning):
			progress.StudyingCount++
		case string(db.CardStateReview):
			progress.MemorizedCount++
		}
		progress.TotalCount++
	}
	return progress, nil
}
