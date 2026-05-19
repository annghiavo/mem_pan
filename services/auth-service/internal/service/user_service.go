package service

import (
	"context"
	"log"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/internal/publisher"
	"mem_pan/services/auth-service/internal/repository"
)

type UpdateProfileParams struct {
	FullName  *string
	AvatarURL *string
	Timezone  *string
}

type ChangePasswordParams struct {
	UserID      uuid.UUID
	OldPassword string
	NewPassword string
}

type ListUsersParams struct {
	Limit        int32
	Offset       int32
	FilterBanned bool
}

type ListUsersResult struct {
	Users []db.User
	Total int64
}

type UserService interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (db.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, params UpdateProfileParams) (db.User, error)
	ChangePassword(ctx context.Context, params ChangePasswordParams) error
	SetUserRole(ctx context.Context, email, role string) (db.User, error)
	ListUsers(ctx context.Context, params ListUsersParams) (ListUsersResult, error)
	BanUser(ctx context.Context, userID uuid.UUID, reason string) (db.User, error)
	UnbanUser(ctx context.Context, userID uuid.UUID) (db.User, error)
}

type userService struct {
	userRepo  repository.UserRepository
	publisher publisher.EventPublisher
}

func NewUserService(userRepo repository.UserRepository, pubs ...publisher.EventPublisher) UserService {
	var pub publisher.EventPublisher = publisher.NewNoopPublisher()
	if len(pubs) > 0 {
		pub = pubs[0]
	}
	return &userService{userRepo: userRepo, publisher: pub}
}

func (s *userService) GetProfile(ctx context.Context, userID uuid.UUID) (db.User, error) {
	return s.userRepo.GetUserByID(ctx, userID)
}

func (s *userService) UpdateProfile(ctx context.Context, userID uuid.UUID, params UpdateProfileParams) (db.User, error) {
	user, err := s.userRepo.UpdateUser(ctx, db.UpdateUserParams{
		UserID:    userID,
		FullName:  domain.NullStr(params.FullName),
		AvatarUrl: domain.NullStr(params.AvatarURL),
		Timezone:  domain.NullStr(params.Timezone),
	})
	if err != nil {
		return db.User{}, err
	}
	if pubErr := s.publisher.PublishUserUpdated(ctx, publisher.UserUpdatedEvent{
		UserID:    user.UserID,
		Username:  user.Username,
		FullName:  user.FullName.String,
		AvatarURL: user.AvatarUrl.String,
	}); pubErr != nil {
		log.Printf("[publisher] user.updated: %v", pubErr)
	}
	return user, nil
}

func (s *userService) SetUserRole(ctx context.Context, email, role string) (db.User, error) {
	return s.userRepo.UpdateUserRole(ctx, db.UpdateUserRoleParams{
		Email: email,
		Role:  role,
	})
}

func (s *userService) ListUsers(ctx context.Context, params ListUsersParams) (ListUsersResult, error) {
	users, err := s.userRepo.ListUsers(ctx, db.ListUsersParams{
		Limit:        params.Limit,
		Offset:       params.Offset,
		FilterBanned: params.FilterBanned,
	})
	if err != nil {
		return ListUsersResult{}, err
	}
	total, err := s.userRepo.CountUsers(ctx, params.FilterBanned)
	if err != nil {
		return ListUsersResult{}, err
	}
	return ListUsersResult{Users: users, Total: total}, nil
}

func (s *userService) BanUser(ctx context.Context, userID uuid.UUID, reason string) (db.User, error) {
	if err := s.userRepo.BanUser(ctx, db.BanUserParams{
		UserID:       userID,
		BannedReason: domain.NullStr(&reason),
	}); err != nil {
		return db.User{}, err
	}
	return s.userRepo.GetUserByID(ctx, userID)
}

func (s *userService) UnbanUser(ctx context.Context, userID uuid.UUID) (db.User, error) {
	if err := s.userRepo.UnbanUser(ctx, userID); err != nil {
		return db.User{}, err
	}
	return s.userRepo.GetUserByID(ctx, userID)
}

func (s *userService) ChangePassword(ctx context.Context, params ChangePasswordParams) error {
	user, err := s.userRepo.GetUserByID(ctx, params.UserID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(params.OldPassword)); err != nil {
		return domain.ErrInvalidCredentials
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(params.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.userRepo.UpdatePassword(ctx, db.UpdatePasswordParams{
		UserID:       params.UserID,
		PasswordHash: string(hashed),
	})
}
