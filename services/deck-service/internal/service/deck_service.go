package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/billingclient"
	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
	"mem_pan/services/deck-service/internal/publisher"
	"mem_pan/services/deck-service/internal/repository"
)

type DeckSettings struct {
	QuizType       string `json:"quiz_type"`
	AnswerSide     string `json:"answer_side"`
	StrictTyping   bool   `json:"strict_typing"`
	PartialCorrect bool   `json:"partial_correct"`
	NewCardsPerDay int32  `json:"new_cards_per_day"`
	ReviewsPerDay  int32  `json:"reviews_per_day"`
}

type CreateDeckParams struct {
	UserID      uuid.UUID
	Name        string
	Description *string
	IsPublic    bool
}

type UpdateDeckParams struct {
	DeckID      uuid.UUID
	UserID      uuid.UUID
	Name        *string
	Description *string
}

type ListDecksParams struct {
	UserID uuid.UUID
	Limit  int32
	Offset int32
}

type ListPublicDecksParams struct {
	UserID      uuid.UUID
	Limit       int32
	Offset      int32
	AccessLevel string
}

type DecksPage struct {
	Decks []db.Deck
	Total int64
}

type DeckStats struct {
	DeckID     uuid.UUID
	TotalCards int64
}

type CreatorProfileParams struct {
	UserID            uuid.UUID
	DisplayName       *string
	Bio               *string
	BankName          *string
	BankAccountNumber *string
	BankAccountName   *string
}

type DeckService interface {
	CreateDeck(ctx context.Context, p CreateDeckParams) (db.Deck, error)
	GetDeck(ctx context.Context, deckID, userID uuid.UUID, publicOK bool) (db.Deck, error)
	ListDecks(ctx context.Context, p ListDecksParams) (DecksPage, error)
	ListPublicDecks(ctx context.Context, p ListPublicDecksParams) (DecksPage, error)
	// ListPublicDecksByIDs returns the public, active decks among the given IDs,
	// preserving the input order. Used to hydrate the trending leaderboard whose
	// ranking is computed by study-service. Missing/private/deleted IDs are
	// silently dropped.
	ListPublicDecksByIDs(ctx context.Context, ids []uuid.UUID) ([]db.Deck, error)
	UpdateDeck(ctx context.Context, p UpdateDeckParams) (db.Deck, error)
	DeleteDeck(ctx context.Context, deckID, userID uuid.UUID) error
	UpdateSettings(ctx context.Context, deckID, userID uuid.UUID, settings DeckSettings) (db.Deck, error)
	UpdateVisibility(ctx context.Context, deckID, userID uuid.UUID, isPublic bool) (db.Deck, error)
	UpdateAccessLevel(ctx context.Context, deckID, userID uuid.UUID, accessLevel string) (db.Deck, error)
	CloneDeck(ctx context.Context, sourceDeckID, newOwnerID uuid.UUID, role ...string) (db.Deck, error)
	GetStats(ctx context.Context, deckID, userID uuid.UUID) (DeckStats, error)
	AdminUpdateDeckStatus(ctx context.Context, deckID uuid.UUID, status string) (db.Deck, error)
	AdminReviewDeckPlus(ctx context.Context, deckID uuid.UUID, plusStatus string) (db.Deck, error)
	AdminListDecks(ctx context.Context, limit, offset int32, statusFilter string) (DecksPage, error)
	UpsertCreatorProfile(ctx context.Context, p CreatorProfileParams) (db.CreatorProfile, error)
	GetCreatorProfile(ctx context.Context, userID uuid.UUID) (db.CreatorProfile, error)
	FollowCreator(ctx context.Context, creatorID, followerID uuid.UUID) error
	UpsertDeckReview(ctx context.Context, deckID, userID uuid.UUID, rating int32) (db.DeckReview, db.Deck, error)
	ListDeckReviews(ctx context.Context, deckID uuid.UUID, limit, offset int32) ([]db.DeckReview, error)
}

type deckService struct {
	deckRepo repository.DeckRepository
	cardRepo repository.CardRepository
	billing  billingclient.Client
	pub      publisher.EventPublisher
}

func NewDeckService(deckRepo repository.DeckRepository, cardRepo repository.CardRepository, opts ...interface{}) DeckService {
	var pub publisher.EventPublisher = publisher.NewNoopPublisher()
	var billing billingclient.Client
	for _, opt := range opts {
		switch v := opt.(type) {
		case publisher.EventPublisher:
			pub = v
		case billingclient.Client:
			billing = v
		}
	}
	return &deckService{deckRepo: deckRepo, cardRepo: cardRepo, billing: billing, pub: pub}
}

func (s *deckService) CreateDeck(ctx context.Context, p CreateDeckParams) (db.Deck, error) {
	deck, err := s.deckRepo.CreateDeck(ctx, db.CreateDeckParams{
		UserID:      p.UserID,
		Name:        p.Name,
		Description: nullStr(p.Description),
		IsPublic:    p.IsPublic,
	})
	if err != nil {
		return db.Deck{}, err
	}
	if pubErr := s.pub.PublishDeckCreated(ctx, publisher.DeckCreatedEvent{
		DeckID:      deck.DeckID.String(),
		UserID:      deck.UserID.String(),
		DeckName:    deck.Name,
		Description: nullStrVal(deck.Description),
		IsPublic:    deck.IsPublic,
		CardCount:   deck.CardCount,
		CreatedAt:   deck.CreatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] deck.created: %v", pubErr)
	}
	return deck, nil
}

func (s *deckService) GetDeck(ctx context.Context, deckID, userID uuid.UUID, publicOK bool) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.Status == string(db.ContentStatusDeleted) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	if deck.UserID != userID {
		if !publicOK || !deck.IsPublic || deck.AccessLevel == db.DeckAccessLevelPrivate {
			return db.Deck{}, domain.ErrForbidden
		}
		if deck.AccessLevel == db.DeckAccessLevelPlus && deck.PlusStatus != db.DeckPlusStatusApproved {
			return db.Deck{}, domain.ErrForbidden
		}
	}
	return deck, nil
}

func (s *deckService) ListPublicDecksByIDs(ctx context.Context, ids []uuid.UUID) ([]db.Deck, error) {
	out := make([]db.Deck, 0, len(ids))
	for _, id := range ids {
		deck, err := s.deckRepo.GetDeckByID(ctx, id)
		if err != nil {
			if errors.Is(err, domain.ErrDeckNotFound) {
				continue
			}
			return nil, err
		}
		if deck.IsPublic && deck.Status == string(db.ContentStatusActive) {
			out = append(out, deck)
		}
	}
	return out, nil
}

func (s *deckService) ListDecks(ctx context.Context, p ListDecksParams) (DecksPage, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	decks, err := s.deckRepo.ListDecksByUser(ctx, db.ListDecksByUserParams{
		UserID: p.UserID,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return DecksPage{}, err
	}
	total, err := s.deckRepo.CountDecksByUser(ctx, p.UserID)
	if err != nil {
		return DecksPage{}, err
	}
	return DecksPage{Decks: decks, Total: total}, nil
}

func (s *deckService) ListPublicDecks(ctx context.Context, p ListPublicDecksParams) (DecksPage, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.UserID != uuid.Nil {
		decks, err := s.deckRepo.ListPublicDecksByUser(ctx, db.ListPublicDecksByUserParams{
			UserID: p.UserID,
			Limit:  p.Limit,
			Offset: p.Offset,
		})
		if err != nil {
			return DecksPage{}, err
		}
		total, err := s.deckRepo.CountPublicDecksByUser(ctx, p.UserID)
		if err != nil {
			return DecksPage{}, err
		}
		return DecksPage{Decks: decks, Total: total}, nil
	}
	decks, err := s.deckRepo.ListPublicDecks(ctx, db.ListPublicDecksParams{
		Limit:       p.Limit,
		Offset:      p.Offset,
		AccessLevel: accessLevelFilter(p.AccessLevel),
	})
	if err != nil {
		return DecksPage{}, err
	}
	total, err := s.deckRepo.CountPublicDecks(ctx, accessLevelFilter(p.AccessLevel))
	if err != nil {
		return DecksPage{}, err
	}
	return DecksPage{Decks: decks, Total: total}, nil
}

func (s *deckService) UpdateDeck(ctx context.Context, p UpdateDeckParams) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, p.DeckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.UserID != p.UserID {
		return db.Deck{}, domain.ErrForbidden
	}
	updated, err := s.deckRepo.UpdateDeck(ctx, db.UpdateDeckParams{
		DeckID:      p.DeckID,
		UserID:      p.UserID,
		Name:        nullStr(p.Name),
		Description: nullStr(p.Description),
	})
	if err != nil {
		return db.Deck{}, err
	}
	if pubErr := s.pub.PublishDeckUpdated(ctx, publisher.DeckUpdatedEvent{
		DeckID:      updated.DeckID.String(),
		UserID:      updated.UserID.String(),
		DeckName:    updated.Name,
		Description: nullStrVal(updated.Description),
		IsPublic:    updated.IsPublic,
		CardCount:   updated.CardCount,
		UpdatedAt:   updated.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] deck.updated: %v", pubErr)
	}
	return updated, nil
}

func (s *deckService) DeleteDeck(ctx context.Context, deckID, userID uuid.UUID) error {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return err
	}
	if deck.UserID != userID {
		return domain.ErrForbidden
	}
	if err := s.deckRepo.SoftDeleteDeck(ctx, db.SoftDeleteDeckParams{
		DeckID: deckID,
		UserID: userID,
	}); err != nil {
		return err
	}
	if pubErr := s.pub.PublishDeckDeleted(ctx, publisher.DeckDeletedEvent{
		DeckID: deckID.String(),
		UserID: userID.String(),
	}); pubErr != nil {
		log.Printf("[publisher] deck.deleted: %v", pubErr)
	}
	return nil
}

func (s *deckService) UpdateSettings(ctx context.Context, deckID, userID uuid.UUID, settings DeckSettings) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.UserID != userID {
		return db.Deck{}, domain.ErrForbidden
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return db.Deck{}, fmt.Errorf("marshal settings: %w", err)
	}
	return s.deckRepo.UpdateDeckSettings(ctx, db.UpdateDeckSettingsParams{
		DeckID:   deckID,
		Settings: raw,
	})
}

func (s *deckService) UpdateVisibility(ctx context.Context, deckID, userID uuid.UUID, isPublic bool) (db.Deck, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.UserID != userID {
		return db.Deck{}, domain.ErrForbidden
	}
	updated, err := s.deckRepo.UpdateDeckVisibility(ctx, db.UpdateDeckVisibilityParams{
		DeckID:   deckID,
		UserID:   userID,
		IsPublic: isPublic,
	})
	if err != nil {
		return db.Deck{}, err
	}
	if pubErr := s.pub.PublishDeckUpdated(ctx, publisher.DeckUpdatedEvent{
		DeckID:      updated.DeckID.String(),
		UserID:      updated.UserID.String(),
		DeckName:    updated.Name,
		Description: nullStrVal(updated.Description),
		IsPublic:    updated.IsPublic,
		CardCount:   updated.CardCount,
		UpdatedAt:   updated.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] deck.updated (visibility): %v", pubErr)
	}
	return updated, nil
}

func (s *deckService) UpdateAccessLevel(ctx context.Context, deckID, userID uuid.UUID, accessLevel string) (db.Deck, error) {
	level, err := parseDeckAccessLevel(accessLevel)
	if err != nil {
		return db.Deck{}, err
	}
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.Deck{}, err
	}
	if deck.UserID != userID {
		return db.Deck{}, domain.ErrForbidden
	}
	updated, err := s.deckRepo.UpdateDeckAccessLevel(ctx, db.UpdateDeckAccessLevelParams{
		DeckID:      deckID,
		UserID:      userID,
		AccessLevel: level,
	})
	if err != nil {
		return db.Deck{}, err
	}
	if pubErr := s.pub.PublishDeckUpdated(ctx, publisher.DeckUpdatedEvent{
		DeckID:      updated.DeckID.String(),
		UserID:      updated.UserID.String(),
		DeckName:    updated.Name,
		Description: nullStrVal(updated.Description),
		IsPublic:    updated.IsPublic,
		CardCount:   updated.CardCount,
		UpdatedAt:   updated.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] deck.updated (access): %v", pubErr)
	}
	return updated, nil
}

func (s *deckService) CloneDeck(ctx context.Context, sourceDeckID, newOwnerID uuid.UUID, role ...string) (db.Deck, error) {
	src, err := s.deckRepo.GetDeckByID(ctx, sourceDeckID)
	if err != nil {
		return db.Deck{}, err
	}
	if src.Status == string(db.ContentStatusDeleted) {
		return db.Deck{}, domain.ErrDeckNotFound
	}
	if err := requireFullDeckAccess(ctx, src, newOwnerID, firstRole(role), s.billing); err != nil {
		return db.Deck{}, err
	}
	clonedName := "Copy of " + src.Name
	newDeck, newCards, err := s.deckRepo.CloneDeck(ctx, src, newOwnerID, clonedName)
	if err != nil {
		return db.Deck{}, err
	}
	if pubErr := s.pub.PublishDeckCreated(ctx, publisher.DeckCreatedEvent{
		DeckID:      newDeck.DeckID.String(),
		UserID:      newDeck.UserID.String(),
		DeckName:    newDeck.Name,
		Description: nullStrVal(newDeck.Description),
		IsPublic:    newDeck.IsPublic,
		CardCount:   newDeck.CardCount,
		CreatedAt:   newDeck.CreatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] deck.created (clone): %v", pubErr)
	}
	for _, c := range newCards {
		if pubErr := s.pub.PublishCardCreated(ctx, publisher.CardCreatedEvent{
			CardID:       c.CardID.String(),
			DeckID:       c.DeckID.String(),
			UserID:       c.UserID.String(),
			NoteID:       c.NoteID.String(),
			ContentFront: c.ContentFront,
			ContentBack:  c.ContentBack,
			ImageURL:     c.ImageUrl.String,
			CreatedAt:    c.CreatedAt,
		}); pubErr != nil {
			log.Printf("[publisher] card.created (clone): %v", pubErr)
		}
	}
	return newDeck, nil
}

func (s *deckService) AdminUpdateDeckStatus(ctx context.Context, deckID uuid.UUID, statusStr string) (db.Deck, error) {
	deck, err := s.deckRepo.AdminUpdateDeckStatus(ctx, db.AdminUpdateDeckStatusParams{
		DeckID: deckID,
		Status: statusStr,
	})
	if err != nil {
		return db.Deck{}, err
	}
	if pubErr := s.pub.PublishDeckUpdated(ctx, publisher.DeckUpdatedEvent{
		DeckID:      deck.DeckID.String(),
		UserID:      deck.UserID.String(),
		DeckName:    deck.Name,
		Description: nullStrVal(deck.Description),
		IsPublic:    deck.IsPublic,
		CardCount:   deck.CardCount,
		UpdatedAt:   deck.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] deck.updated (admin status): %v", pubErr)
	}
	return deck, nil
}

func (s *deckService) AdminReviewDeckPlus(ctx context.Context, deckID uuid.UUID, plusStatus string) (db.Deck, error) {
	status, err := parseDeckPlusStatus(plusStatus)
	if err != nil {
		return db.Deck{}, err
	}
	deck, err := s.deckRepo.AdminReviewDeckPlus(ctx, db.AdminReviewDeckPlusParams{
		DeckID:     deckID,
		PlusStatus: status,
	})
	if err != nil {
		return db.Deck{}, err
	}
	if pubErr := s.pub.PublishDeckUpdated(ctx, publisher.DeckUpdatedEvent{
		DeckID:      deck.DeckID.String(),
		UserID:      deck.UserID.String(),
		DeckName:    deck.Name,
		Description: nullStrVal(deck.Description),
		IsPublic:    deck.IsPublic,
		CardCount:   deck.CardCount,
		UpdatedAt:   deck.UpdatedAt,
	}); pubErr != nil {
		log.Printf("[publisher] deck.updated (plus review): %v", pubErr)
	}
	return deck, nil
}

func (s *deckService) AdminListDecks(ctx context.Context, limit, offset int32, statusFilter string) (DecksPage, error) {
	var filter sql.NullString
	if statusFilter != "" {
		filter = sql.NullString{String: statusFilter, Valid: true}
	}
	decks, err := s.deckRepo.AdminListDecks(ctx, db.AdminListDecksParams{
		Limit:        limit,
		Offset:       offset,
		StatusFilter: filter,
	})
	if err != nil {
		return DecksPage{}, err
	}
	total, err := s.deckRepo.AdminCountDecks(ctx, filter)
	if err != nil {
		return DecksPage{}, err
	}
	return DecksPage{Decks: decks, Total: total}, nil
}

func (s *deckService) UpsertCreatorProfile(ctx context.Context, p CreatorProfileParams) (db.CreatorProfile, error) {
	return s.deckRepo.UpsertCreatorProfile(ctx, db.UpsertCreatorProfileParams{
		UserID:            p.UserID,
		DisplayName:       nullStr(p.DisplayName),
		Bio:               nullStr(p.Bio),
		BankName:          nullStr(p.BankName),
		BankAccountNumber: nullStr(p.BankAccountNumber),
		BankAccountName:   nullStr(p.BankAccountName),
	})
}

func (s *deckService) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (db.CreatorProfile, error) {
	return s.deckRepo.GetCreatorProfile(ctx, userID)
}

func (s *deckService) FollowCreator(ctx context.Context, creatorID, followerID uuid.UUID) error {
	if creatorID == followerID {
		return domain.ErrForbidden
	}
	if _, err := s.deckRepo.GetCreatorProfile(ctx, creatorID); err != nil {
		return err
	}
	return s.deckRepo.FollowCreator(ctx, creatorID, followerID)
}

func (s *deckService) UpsertDeckReview(ctx context.Context, deckID, userID uuid.UUID, rating int32) (db.DeckReview, db.Deck, error) {
	if rating < 1 || rating > 5 {
		return db.DeckReview{}, db.Deck{}, domain.ErrReviewNotAllowed
	}
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return db.DeckReview{}, db.Deck{}, err
	}
	if deck.UserID == userID {
		return db.DeckReview{}, db.Deck{}, domain.ErrReviewNotAllowed
	}
	if deck.AccessLevel == db.DeckAccessLevelPlus {
		if deck.PlusStatus != db.DeckPlusStatusApproved {
			return db.DeckReview{}, db.Deck{}, domain.ErrReviewNotAllowed
		}
		if s.billing == nil {
			return db.DeckReview{}, db.Deck{}, domain.ErrPlusRequired
		}
		active, err := s.billing.CheckPlusAccess(ctx, userID)
		if err != nil {
			return db.DeckReview{}, db.Deck{}, err
		}
		if !active {
			return db.DeckReview{}, db.Deck{}, domain.ErrPlusRequired
		}
	}
	review, err := s.deckRepo.UpsertDeckReview(ctx, db.UpsertDeckReviewParams{
		DeckID:  deckID,
		UserID:  userID,
		Rating:  int16(rating),
	})
	if err != nil {
		return db.DeckReview{}, db.Deck{}, err
	}
	updatedDeck, err := s.deckRepo.RebuildDeckRating(ctx, deckID)
	if err != nil {
		return db.DeckReview{}, db.Deck{}, err
	}
	return review, updatedDeck, nil
}

func (s *deckService) ListDeckReviews(ctx context.Context, deckID uuid.UUID, limit, offset int32) ([]db.DeckReview, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.deckRepo.ListDeckReviews(ctx, db.ListDeckReviewsParams{
		DeckID: deckID,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *deckService) GetStats(ctx context.Context, deckID, userID uuid.UUID) (DeckStats, error) {
	deck, err := s.deckRepo.GetDeckByID(ctx, deckID)
	if err != nil {
		return DeckStats{}, err
	}
	if deck.UserID != userID && !deck.IsPublic {
		return DeckStats{}, domain.ErrForbidden
	}
	total, err := s.cardRepo.CountCardsByDeck(ctx, deckID)
	if err != nil {
		return DeckStats{}, err
	}
	return DeckStats{DeckID: deckID, TotalCards: total}, nil
}
