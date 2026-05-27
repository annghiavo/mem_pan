package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/deckclient"
	"mem_pan/services/study-service/internal/domain"
	"mem_pan/services/study-service/internal/fsrs"
	"mem_pan/services/study-service/internal/publisher"
	"mem_pan/services/study-service/internal/repository"
)

const (
	defaultNewCardsLimit    = int32(20)
	defaultReviewCardsLimit = int32(200)

	// Trending-window defaults for TopDecksByLearners.
	defaultTrendingWindowDays = int32(7)
	defaultTopDecksLimit      = int32(10)
	maxTopDecksLimit          = int32(100)
)

// DeckLearners is one row of the trending leaderboard: a deck and the number of
// distinct users who studied it within the requested window.
type DeckLearners struct {
	DeckID   uuid.UUID
	Learners int64
}

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
	// Timezone is the user's IANA timezone forwarded onto the card.reviewed
	// event so stats-service can compute the streak day and activity bucket
	// in the user's local time. Empty falls back to UTC.
	Timezone string
}

type SessionResult struct {
	Session db.StudySession
	Cards   []db.SessionCard
}

type RecentDeck struct {
	DeckID         uuid.UUID
	LastAccessedAt time.Time
}

type ProgressTag struct {
	Label   string
	Count   int32
	CardIDs []uuid.UUID
}

type DeckProgress struct {
	DeckID         uuid.UUID
	NewCount       int32
	LearnCount     int32
	MemorizedCount int32
	TotalCount     int32
	Tags           []ProgressTag
	// NextReviewDate is the soonest next_review_date among non-new cards, or
	// nil when the deck has no scheduled cards. DueNow counts non-new cards
	// already due (next_review_date <= now).
	NextReviewDate *time.Time
	DueNow         int32
}

type StudyService interface {
	StartSession(ctx context.Context, p StartSessionParams) (*SessionResult, error)
	GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*SessionResult, error)
	ReviewCard(ctx context.Context, p ReviewCardParams) (db.UserCard, error)
	FinishSession(ctx context.Context, sessionID, userID uuid.UUID) (*SessionResult, error)
	GetDueCards(ctx context.Context, userID uuid.UUID, deckID *uuid.UUID) ([]db.UserCard, error)
	// CountDueByEndOfDay returns the number of non-new cards whose
	// next_review_date is on or before the end of "today" in the given IANA
	// timezone. Used by notification-service reminder crons.
	CountDueByEndOfDay(ctx context.Context, userID uuid.UUID, timezone string) (int32, error)
	GetRecentSessionCards(ctx context.Context, userID uuid.UUID) (*SessionResult, error)
	GetRecentDecks(ctx context.Context, userID uuid.UUID) ([]RecentDeck, error)
	GetDeckProgress(ctx context.Context, userID, deckID uuid.UUID) (*DeckProgress, error)
	// CountDeckLearners returns the number of distinct users who have ever
	// started a study session on the deck (the owner is included). Consumed by
	// deck-service to show a learner count on the deck detail page.
	CountDeckLearners(ctx context.Context, deckID uuid.UUID) (int64, error)
	// TopDecksByLearners ranks decks by distinct learners active within the last
	// windowDays days (trending). Consumed by deck-service, which filters the
	// result down to public decks. Returns at most limit rows.
	TopDecksByLearners(ctx context.Context, windowDays, limit int32) ([]DeckLearners, error)
}

type studyService struct {
	userCardRepo    repository.UserCardRepository
	sessionRepo     repository.StudySessionRepository
	sessionCardRepo repository.SessionCardRepository
	revlogRepo      repository.RevlogRepository
	weightsRepo     repository.FsrsWeightsRepository
	deckClient      deckclient.Client
	pub             publisher.EventPublisher
}

func NewStudyService(
	userCardRepo repository.UserCardRepository,
	sessionRepo repository.StudySessionRepository,
	sessionCardRepo repository.SessionCardRepository,
	revlogRepo repository.RevlogRepository,
	weightsRepo repository.FsrsWeightsRepository,
	deckClient deckclient.Client,
	pubs ...publisher.EventPublisher,
) StudyService {
	var pub publisher.EventPublisher = publisher.NewNoopPublisher()
	if len(pubs) > 0 {
		pub = pubs[0]
	}
	return &studyService{
		userCardRepo:    userCardRepo,
		sessionRepo:     sessionRepo,
		sessionCardRepo: sessionCardRepo,
		revlogRepo:      revlogRepo,
		weightsRepo:     weightsRepo,
		deckClient:      deckClient,
		pub:             pub,
	}
}

func (s *studyService) StartSession(ctx context.Context, p StartSessionParams) (*SessionResult, error) {
	// Always start a fresh session. If an old one is still 'ongoing' (the user
	// abandoned it earlier or the client never finished it), close it and move
	// on. Cards already reviewed in the old session have had their FSRS state
	// updated; cards that were skipped are still due and will be picked up by
	// the new session's due-card selection below. Resuming would just hand
	// back stale already-reviewed session_card rows that 409 on /review.
	existing, err := s.sessionRepo.GetOngoingSessionByDeck(ctx, db.GetOngoingSessionByDeckParams{
		UserID: p.UserID,
		DeckID: p.DeckID,
	})
	if err == nil {
		if _, finErr := s.sessionRepo.AbandonStudySession(ctx, existing.SessionID); finErr != nil {
			return nil, finErr
		}
	} else if !errors.Is(err, domain.ErrSessionNotFound) {
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

	// IncrementCompletedCards atomically flips status to 'completed' on the
	// final card (see study_session.sql) so the session can't get stuck in
	// 'ongoing' state with every card already graded.
	_, err = s.sessionRepo.IncrementCompletedCards(ctx, p.SessionID)
	if err != nil {
		return db.UserCard{}, err
	}

	if pubErr := s.pub.PublishCardReviewed(ctx, publisher.CardReviewedEvent{
		UserID:         p.UserID.String(),
		CardID:         uc.CardID.String(),
		DeckID:         uc.DeckID.String(),
		Rating:         p.Rating,
		DurationMs:     int64(p.DurationMS),
		StateBefore:    uc.State,
		StateAfter:     updatedUC.State,
		StabilityAfter: updatedUC.Stability,
		IsNewCard:      uc.State == string(db.CardStateNew),
		ReviewTime:     now,
		Timezone:       p.Timezone,
	}); pubErr != nil {
		log.Printf("[publisher] card.reviewed: %v", pubErr)
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
	// /finish is idempotent: if the session already auto-completed when the
	// last card's review hit total_cards (see IncrementCompletedCards), the
	// client's follow-up explicit /finish call returns the existing session
	// instead of erroring.
	if session.Status != string(db.SessionStatusOngoing) {
		cards, err := s.sessionCardRepo.ListSessionCards(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		return &SessionResult{Session: session, Cards: cards}, nil
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

func (s *studyService) CountDeckLearners(ctx context.Context, deckID uuid.UUID) (int64, error) {
	return s.sessionRepo.CountDeckLearners(ctx, deckID)
}

func (s *studyService) TopDecksByLearners(ctx context.Context, windowDays, limit int32) ([]DeckLearners, error) {
	if windowDays <= 0 {
		windowDays = defaultTrendingWindowDays
	}
	if limit <= 0 {
		limit = defaultTopDecksLimit
	}
	if limit > maxTopDecksLimit {
		limit = maxTopDecksLimit
	}
	since := time.Now().AddDate(0, 0, -int(windowDays))
	rows, err := s.sessionRepo.TopDecksByLearners(ctx, db.TopDecksByLearnersParams{
		LastAccessedAt: since,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]DeckLearners, len(rows))
	for i, r := range rows {
		out[i] = DeckLearners{DeckID: r.DeckID, Learners: r.Learners}
	}
	return out, nil
}

func (s *studyService) CountDueByEndOfDay(ctx context.Context, userID uuid.UUID, timezone string) (int32, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil || timezone == "" {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	// End-of-day in user's tz = next day's 00:00 - 1ns.
	endLocal := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), loc)
	return s.userCardRepo.CountDueByEndOfDay(ctx, userID, endLocal.UTC())
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
	newIDs := make([]uuid.UUID, 0)
	learnIDs := make([]uuid.UUID, 0)
	memorizedIDs := make([]uuid.UUID, 0)

	now := time.Now()
	progress := &DeckProgress{DeckID: deckID}
	for _, c := range cards {
		switch c.State {
		case string(db.CardStateNew):
			progress.NewCount++
			newIDs = append(newIDs, c.CardID)
		case string(db.CardStateLearning), string(db.CardStateRelearning):
			progress.LearnCount++
			learnIDs = append(learnIDs, c.CardID)
		case string(db.CardStateReview):
			progress.MemorizedCount++
			memorizedIDs = append(memorizedIDs, c.CardID)
		}
		progress.TotalCount++

		// Track the soonest review across non-new cards so the client can show
		// a deck-level countdown. Cards already past due bump DueNow.
		if c.State != string(db.CardStateNew) {
			if c.NextReviewDate.Before(now) || c.NextReviewDate.Equal(now) {
				progress.DueNow++
			}
			if progress.NextReviewDate == nil || c.NextReviewDate.Before(*progress.NextReviewDate) {
				t := c.NextReviewDate
				progress.NextReviewDate = &t
			}
		}
	}
	progress.Tags = []ProgressTag{
		{Label: "new", Count: progress.NewCount, CardIDs: newIDs},
		{Label: "learning", Count: progress.LearnCount, CardIDs: learnIDs},
		{Label: "memorized", Count: progress.MemorizedCount, CardIDs: memorizedIDs},
	}
	return progress, nil
}
