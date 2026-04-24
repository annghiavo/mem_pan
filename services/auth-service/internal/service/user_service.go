package service

import (
	"context"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/internal/repository"
)

type UpdateProfileParams struct {
	FullName  *string
	AvatarURL *string
}

type ChangePasswordParams struct {
	UserID      uuid.UUID
	OldPassword string
	NewPassword string
}

type UserService interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (db.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, params UpdateProfileParams) (db.User, error)
	ChangePassword(ctx context.Context, params ChangePasswordParams) error
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) GetProfile(ctx context.Context, userID uuid.UUID) (db.User, error) {
	return s.userRepo.GetUserByID(ctx, userID)
}

func (s *userService) UpdateProfile(ctx context.Context, userID uuid.UUID, params UpdateProfileParams) (db.User, error) {
	return s.userRepo.UpdateUser(ctx, db.UpdateUserParams{
		UserID:    userID,
		FullName:  domain.NullStr(params.FullName),
		AvatarUrl: domain.NullStr(params.AvatarURL),
	})
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
