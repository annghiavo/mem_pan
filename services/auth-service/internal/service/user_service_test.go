package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/internal/mock"
)

func TestGetProfile_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	user := makeUser(userID)

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)

	svc := NewUserService(userRepo)
	result, err := svc.GetProfile(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.UserID != userID {
		t.Errorf("expected userID %v, got %v", userID, result.UserID)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(db.User{}, domain.ErrUserNotFound)

	svc := NewUserService(userRepo)
	_, err := svc.GetProfile(ctx, userID)

	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUpdateProfile_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	fullName := "Updated Name"
	user := makeUser(userID)
	user.FullName.String = fullName
	user.FullName.Valid = true

	userRepo.EXPECT().UpdateUser(ctx, db.UpdateUserParams{
		UserID:    userID,
		FullName:  domain.NullStr(&fullName),
		AvatarUrl: domain.NullStr(nil),
	}).Return(user, nil)

	svc := NewUserService(userRepo)
	result, err := svc.UpdateProfile(ctx, userID, UpdateProfileParams{FullName: &fullName})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.FullName.Valid || result.FullName.String != fullName {
		t.Errorf("expected full name %q, got %q", fullName, result.FullName.String)
	}
}

func TestUpdateProfile_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()

	userRepo.EXPECT().UpdateUser(ctx, gomock.Any()).Return(db.User{}, domain.ErrUserNotFound)

	svc := NewUserService(userRepo)
	_, err := svc.UpdateProfile(ctx, userID, UpdateProfileParams{})

	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestChangePassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	user := makeUser(userID)
	// hash for "password" generated with bcrypt default cost
	user.PasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	userRepo.EXPECT().UpdatePassword(ctx, gomock.Any()).Return(nil)

	svc := NewUserService(userRepo)
	err := svc.ChangePassword(ctx, ChangePasswordParams{
		UserID:      userID,
		OldPassword: "password",
		NewPassword: "newpassword123",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()
	user := makeUser(userID)
	user.PasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)

	svc := NewUserService(userRepo)
	err := svc.ChangePassword(ctx, ChangePasswordParams{
		UserID:      userID,
		OldPassword: "wrongpassword",
		NewPassword: "newpassword123",
	})

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestChangePassword_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	ctx := context.Background()
	userID := uuid.New()

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(db.User{}, domain.ErrUserNotFound)

	svc := NewUserService(userRepo)
	err := svc.ChangePassword(ctx, ChangePasswordParams{
		UserID:      userID,
		OldPassword: "password",
		NewPassword: "newpassword123",
	})

	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

// ensure makeUser is usable in this test file
var _ = time.Now
