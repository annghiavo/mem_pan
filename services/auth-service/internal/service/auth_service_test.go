package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/internal/mock"
	"mem_pan/services/auth-service/internal/token"
)

func newTestAuthService(
	ctrl *gomock.Controller,
	userRepo *mock.MockUserRepository,
	rtRepo *mock.MockRefreshTokenRepository,
	vtRepo *mock.MockVerificationTokenRepository,
	maker *mock.MockMaker,
	pub *mock.MockEventPublisher,
) AuthService {
	return NewAuthService(
		userRepo, rtRepo, vtRepo, maker, pub,
		15*time.Minute, 7*24*time.Hour, 24*time.Hour, time.Hour,
	)
}

func makeUser(id uuid.UUID) db.User {
	return db.User{
		UserID:        id,
		Username:      "testuser",
		Email:         "test@example.com",
		PasswordHash:  "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", // "password"
		Role:          "user",
		IsBanned:      false,
		EmailVerified: false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func makePayload(userID uuid.UUID, tt token.TokenType) *token.Payload {
	p, _ := token.NewPayload(userID, "testuser", "user", time.Hour, tt)
	return p
}

func makeRefreshToken(userID uuid.UUID) db.RefreshToken {
	return db.RefreshToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		TokenHash: "somehash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}
}

// ------- Register -------

func TestRegister_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	user := makeUser(userID)
	ctx := context.Background()

	userRepo.EXPECT().CreateUser(ctx, gomock.Any()).Return(user, nil)
	pub.EXPECT().PublishUserRegistered(ctx, gomock.Any()).Return(nil)
	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	vtRepo.EXPECT().CreateVerificationToken(ctx, userID, gomock.Any(), domain.VerificationTypeEmail, gomock.Any()).Return(db.VerificationToken{}, nil)
	pub.EXPECT().PublishEmailVerificationRequested(ctx, gomock.Any()).Return(nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	result, err := svc.Register(ctx, RegisterParams{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.UserID != userID {
		t.Errorf("expected user ID %v, got %v", userID, result.UserID)
	}
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	ctx := context.Background()
	userRepo.EXPECT().CreateUser(ctx, gomock.Any()).Return(db.User{}, domain.ErrEmailAlreadyExists)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	_, err := svc.Register(ctx, RegisterParams{
		Username: "testuser",
		Email:    "existing@example.com",
		Password: "password123",
	})

	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

// ------- Login -------

func TestLogin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	user := makeUser(userID)
	accessPayload := makePayload(userID, token.TokenTypeAccess)
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	rt := makeRefreshToken(userID)
	ctx := context.Background()

	userRepo.EXPECT().GetUserByEmail(ctx, "test@example.com").Return(user, nil)
	maker.EXPECT().CreateToken(userID, "testuser", "user", 15*time.Minute, token.TokenTypeAccess).Return("access-token", accessPayload, nil)
	maker.EXPECT().CreateToken(userID, "testuser", "user", 7*24*time.Hour, token.TokenTypeRefresh).Return("refresh-token", refreshPayload, nil)
	rtRepo.EXPECT().DeleteExpiredForUser(ctx, userID).Return(nil)
	rtRepo.EXPECT().CreateRefreshToken(ctx, userID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(rt, nil)
	userRepo.EXPECT().UpdateLastLogin(ctx, userID).Return(nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	resp, err := svc.Login(ctx, LoginParams{
		Email:     "test@example.com",
		Password:  "password",
		UserAgent: "Mozilla",
		ClientIP:  "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Tokens.AccessToken != "access-token" {
		t.Errorf("expected access-token, got %s", resp.Tokens.AccessToken)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	ctx := context.Background()
	userRepo.EXPECT().GetUserByEmail(ctx, gomock.Any()).Return(db.User{}, domain.ErrUserNotFound)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	_, err := svc.Login(ctx, LoginParams{Email: "nobody@example.com", Password: "pw"})

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_BannedUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	user := makeUser(userID)
	user.IsBanned = true
	ctx := context.Background()

	userRepo.EXPECT().GetUserByEmail(ctx, gomock.Any()).Return(user, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	_, err := svc.Login(ctx, LoginParams{Email: "test@example.com", Password: "password"})

	if !errors.Is(err, domain.ErrUserBanned) {
		t.Errorf("expected ErrUserBanned, got %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	user := makeUser(userID)
	ctx := context.Background()

	userRepo.EXPECT().GetUserByEmail(ctx, gomock.Any()).Return(user, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	_, err := svc.Login(ctx, LoginParams{Email: "test@example.com", Password: "wrongpassword"})

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

// ------- RefreshToken -------

func TestRefreshToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	user := makeUser(userID)
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	accessPayload := makePayload(userID, token.TokenTypeAccess)
	rt := makeRefreshToken(userID)
	ctx := context.Background()

	maker.EXPECT().VerifyToken("refresh-token", token.TokenTypeRefresh).Return(refreshPayload, nil)
	rtRepo.EXPECT().GetRefreshTokenByHash(ctx, gomock.Any()).Return(rt, nil)
	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	maker.EXPECT().CreateToken(userID, "testuser", "user", 15*time.Minute, token.TokenTypeAccess).Return("new-access-token", accessPayload, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	tokens, err := svc.RefreshToken(ctx, "refresh-token")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tokens.AccessToken != "new-access-token" {
		t.Errorf("expected new-access-token, got %s", tokens.AccessToken)
	}
}

func TestRefreshToken_RevokedToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	rt := makeRefreshToken(userID)
	rt.RevokedAt = sql.NullTime{Time: time.Now(), Valid: true}
	ctx := context.Background()

	maker.EXPECT().VerifyToken("refresh-token", token.TokenTypeRefresh).Return(refreshPayload, nil)
	rtRepo.EXPECT().GetRefreshTokenByHash(ctx, gomock.Any()).Return(rt, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	_, err := svc.RefreshToken(ctx, "refresh-token")

	if !errors.Is(err, domain.ErrTokenRevoked) {
		t.Errorf("expected ErrTokenRevoked, got %v", err)
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	ctx := context.Background()
	maker.EXPECT().VerifyToken("bad-token", token.TokenTypeRefresh).Return(nil, token.ErrInvalidToken)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	_, err := svc.RefreshToken(ctx, "bad-token")

	if !errors.Is(err, token.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

// ------- Logout -------

func TestLogout_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	ctx := context.Background()

	maker.EXPECT().VerifyToken("refresh-token", token.TokenTypeRefresh).Return(refreshPayload, nil)
	rtRepo.EXPECT().RevokeByHash(ctx, gomock.Any()).Return(nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.Logout(ctx, "refresh-token")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLogout_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	ctx := context.Background()
	maker.EXPECT().VerifyToken("bad-token", token.TokenTypeRefresh).Return(nil, token.ErrInvalidToken)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.Logout(ctx, "bad-token")

	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

// ------- VerifyEmail -------

func TestVerifyEmail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		TokenHash: "somehash",
		Type:      domain.VerificationTypeEmail,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	ctx := context.Background()

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)
	vtRepo.EXPECT().MarkUsed(ctx, gomock.Any()).Return(nil)
	userRepo.EXPECT().MarkEmailVerified(ctx, userID).Return(nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.VerifyEmail(ctx, "rawtoken123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestVerifyEmail_AlreadyUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypeEmail,
		ExpiresAt: time.Now().Add(time.Hour),
		UsedAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}
	ctx := context.Background()

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.VerifyEmail(ctx, "used-token")

	if !errors.Is(err, domain.ErrTokenAlreadyUsed) {
		t.Errorf("expected ErrTokenAlreadyUsed, got %v", err)
	}
}

func TestVerifyEmail_ExpiredToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypeEmail,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	ctx := context.Background()

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.VerifyEmail(ctx, "expired-token")

	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyEmail_TokenNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	ctx := context.Background()
	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(db.VerificationToken{}, domain.ErrTokenNotFound)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.VerifyEmail(ctx, "notfound-token")

	if !errors.Is(err, domain.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

// ------- ForgotPassword -------

func TestForgotPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	user := makeUser(userID)
	ctx := context.Background()

	userRepo.EXPECT().GetUserByEmail(ctx, "test@example.com").Return(user, nil)
	vtRepo.EXPECT().CreateVerificationToken(ctx, userID, gomock.Any(), domain.VerificationTypePasswordReset, gomock.Any()).Return(db.VerificationToken{}, nil)
	pub.EXPECT().PublishPasswordResetRequested(ctx, gomock.Any()).Return(nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.ForgotPassword(ctx, "test@example.com")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestForgotPassword_UnknownEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	ctx := context.Background()
	// swallows error to prevent email enumeration
	userRepo.EXPECT().GetUserByEmail(ctx, "unknown@example.com").Return(db.User{}, domain.ErrUserNotFound)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.ForgotPassword(ctx, "unknown@example.com")

	if err != nil {
		t.Fatalf("expected nil (swallowed), got %v", err)
	}
}

// ------- ResetPassword -------

func TestResetPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypePasswordReset,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	ctx := context.Background()

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)
	vtRepo.EXPECT().MarkUsed(ctx, gomock.Any()).Return(nil)
	userRepo.EXPECT().UpdatePassword(ctx, gomock.Any()).Return(nil)
	rtRepo.EXPECT().RevokeAllForUser(ctx, userID).Return(nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.ResetPassword(ctx, "rawtoken", "newpassword123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestResetPassword_WrongTokenType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypeEmail, // wrong type
		ExpiresAt: time.Now().Add(time.Hour),
	}
	ctx := context.Background()

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.ResetPassword(ctx, "rawtoken", "newpassword123")

	if !errors.Is(err, domain.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

// ------- SendEmailVerification -------

func TestSendEmailVerification_AlreadyVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	userID := uuid.New()
	user := makeUser(userID)
	user.EmailVerified = true
	ctx := context.Background()

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.SendEmailVerification(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ------- ResendEmailVerification -------

func TestResendEmailVerification_UnknownEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)

	ctx := context.Background()
	// swallowed for email enumeration protection
	userRepo.EXPECT().GetUserByEmail(ctx, "unknown@example.com").Return(db.User{}, errors.New("not found"))

	svc := newTestAuthService(ctrl, userRepo, rtRepo, vtRepo, maker, pub)
	err := svc.ResendEmailVerification(ctx, "unknown@example.com")

	if err != nil {
		t.Fatalf("expected nil (swallowed), got %v", err)
	}
}
