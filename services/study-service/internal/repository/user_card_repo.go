package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"mem_pan/services/study-service/internal/db"
	"mem_pan/services/study-service/internal/domain"
)

type UserCardRepository interface {
	UpsertUserCard(ctx context.Context, arg db.UpsertUserCardParams) (db.UserCard, error)
	GetUserCardByID(ctx context.Context, id uuid.UUID) (db.UserCard, error)
	GetUserCardByUserAndCard(ctx context.Context, arg db.GetUserCardByUserAndCardParams) (db.UserCard, error)
	UpdateUserCardFSRS(ctx context.Context, arg db.UpdateUserCardFSRSParams) (db.UserCard, error)
	ListDueUserCards(ctx context.Context, arg db.ListDueUserCardsParams) ([]db.UserCard, error)
	ListDueUserCardsByDeck(ctx context.Context, arg db.ListDueUserCardsByDeckParams) ([]db.UserCard, error)
	ListNewUserCardsByDeck(ctx context.Context, arg db.ListNewUserCardsByDeckParams) ([]db.UserCard, error)
	ListUserCardsByDeck(ctx context.Context, arg db.ListUserCardsByDeckParams) ([]db.UserCard, error)
}

type userCardRepository struct {
	q *db.Queries
}

func NewUserCardRepository(database *sql.DB) UserCardRepository {
	return &userCardRepository{q: db.New(database)}
}

func (r *userCardRepository) UpsertUserCard(ctx context.Context, arg db.UpsertUserCardParams) (db.UserCard, error) {
	return r.q.UpsertUserCard(ctx, arg)
}

func (r *userCardRepository) GetUserCardByID(ctx context.Context, id uuid.UUID) (db.UserCard, error) {
	uc, err := r.q.GetUserCardByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.UserCard{}, domain.ErrSessionNotFound
	}
	return uc, err
}

func (r *userCardRepository) GetUserCardByUserAndCard(ctx context.Context, arg db.GetUserCardByUserAndCardParams) (db.UserCard, error) {
	return r.q.GetUserCardByUserAndCard(ctx, arg)
}

func (r *userCardRepository) UpdateUserCardFSRS(ctx context.Context, arg db.UpdateUserCardFSRSParams) (db.UserCard, error) {
	return r.q.UpdateUserCardFSRS(ctx, arg)
}

func (r *userCardRepository) ListDueUserCards(ctx context.Context, arg db.ListDueUserCardsParams) ([]db.UserCard, error) {
	return r.q.ListDueUserCards(ctx, arg)
}

func (r *userCardRepository) ListDueUserCardsByDeck(ctx context.Context, arg db.ListDueUserCardsByDeckParams) ([]db.UserCard, error) {
	return r.q.ListDueUserCardsByDeck(ctx, arg)
}

func (r *userCardRepository) ListNewUserCardsByDeck(ctx context.Context, arg db.ListNewUserCardsByDeckParams) ([]db.UserCard, error) {
	return r.q.ListNewUserCardsByDeck(ctx, arg)
}

func (r *userCardRepository) ListUserCardsByDeck(ctx context.Context, arg db.ListUserCardsByDeckParams) ([]db.UserCard, error) {
	return r.q.ListUserCardsByDeck(ctx, arg)
}
