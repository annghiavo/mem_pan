package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
)

type UserRepository interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetUserByUsername(ctx context.Context, username string) (db.User, error)
	UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.User, error)
	UpdateUserRole(ctx context.Context, arg db.UpdateUserRoleParams) (db.User, error)
	UpdatePassword(ctx context.Context, arg db.UpdatePasswordParams) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateUserPlusStatus(ctx context.Context, id uuid.UUID, isPlus bool) error
	MarkEmailVerified(ctx context.Context, id uuid.UUID) error
	BanUser(ctx context.Context, arg db.BanUserParams) error
	UnbanUser(ctx context.Context, id uuid.UUID) error
	ListUsers(ctx context.Context, arg db.ListUsersParams) ([]db.User, error)
	CountUsers(ctx context.Context, filterBanned bool) (int64, error)
}

type userRepository struct {
	q *db.Queries
}

func NewUserRepository(database *sql.DB) UserRepository {
	return &userRepository{q: db.New(database)}
}

func (r *userRepository) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	u, err := r.q.CreateUser(ctx, arg)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_email_key" {
				return db.User{}, domain.ErrEmailAlreadyExists
			}
			return db.User{}, domain.ErrUsernameAlreadyExists
		}
		return db.User{}, err
	}
	return u, nil
}

func (r *userRepository) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	u, err := r.q.GetUserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	u, err := r.q.GetUserByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *userRepository) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	u, err := r.q.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *userRepository) UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.User, error) {
	u, err := r.q.UpdateUser(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *userRepository) UpdateUserRole(ctx context.Context, arg db.UpdateUserRoleParams) (db.User, error) {
	u, err := r.q.UpdateUserRole(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *userRepository) UpdatePassword(ctx context.Context, arg db.UpdatePasswordParams) error {
	return r.q.UpdatePassword(ctx, arg)
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	return r.q.UpdateLastLogin(ctx, id)
}

func (r *userRepository) UpdateUserPlusStatus(ctx context.Context, id uuid.UUID, isPlus bool) error {
	return r.q.UpdateUserPlusStatus(ctx, db.UpdateUserPlusStatusParams{
		UserID: id,
		IsPlus: isPlus,
	})
}

func (r *userRepository) MarkEmailVerified(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkEmailVerified(ctx, id)
}

func (r *userRepository) BanUser(ctx context.Context, arg db.BanUserParams) error {
	return r.q.BanUser(ctx, arg)
}

func (r *userRepository) UnbanUser(ctx context.Context, id uuid.UUID) error {
	return r.q.UnbanUser(ctx, id)
}

func (r *userRepository) ListUsers(ctx context.Context, arg db.ListUsersParams) ([]db.User, error) {
	return r.q.ListUsers(ctx, arg)
}

func (r *userRepository) CountUsers(ctx context.Context, filterBanned bool) (int64, error) {
	return r.q.CountUsers(ctx, filterBanned)
}
