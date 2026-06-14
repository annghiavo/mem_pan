package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/internal/mock"
	"mem_pan/services/auth-service/internal/token"
)

var testPasswordHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	return string(h)
}()

const (
	testAccessDur      = 7 * 24 * time.Hour
	testRefreshDur     = 30 * 24 * time.Hour
	testVerifyTokenDur = 24 * time.Hour
	testResetTokenDur  = time.Hour
)

func makeUser(id uuid.UUID) db.User {
	return db.User{
		UserID:        id,
		Username:      "testuser",
		Email:         "test@example.com",
		PasswordHash:  testPasswordHash,
		Role:          "user",
		IsBanned:      false,
		EmailVerified: false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func makePayload(userID uuid.UUID, tt token.TokenType) *token.Payload {
	p, _ := token.NewPayload(userID, "testuser", "user", false, time.Hour, tt)
	return p
}

func makeRefreshToken(userID uuid.UUID) db.RefreshToken {
	return db.RefreshToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		TokenHash: "somehash",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
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
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)

	userRepo.EXPECT().CreateUser(ctx, gomock.Any()).Return(user, nil)
	pub.EXPECT().PublishUserRegistered(ctx, gomock.Any()).Return(nil)
	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	vtRepo.EXPECT().CreateVerificationToken(ctx, userID, gomock.Any(), domain.VerificationTypeEmail, gomock.Any()).Return(db.VerificationToken{}, nil)
	pub.EXPECT().PublishEmailVerificationRequested(ctx, gomock.Any()).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)
	user.EmailVerified = true
	accessPayload := makePayload(userID, token.TokenTypeAccess)
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	rt := makeRefreshToken(userID)

	userRepo.EXPECT().GetUserByEmail(ctx, "test@example.com").Return(user, nil)
	maker.EXPECT().CreateToken(userID, "testuser", "user", false, testAccessDur, token.TokenTypeAccess).Return("access-token", accessPayload, nil)
	maker.EXPECT().CreateToken(userID, "testuser", "user", false, testRefreshDur, token.TokenTypeRefresh).Return("refresh-token", refreshPayload, nil)
	rtRepo.EXPECT().DeleteExpiredForUser(ctx, userID).Return(nil)
	rtRepo.EXPECT().CreateRefreshToken(ctx, userID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(rt, nil)
	userRepo.EXPECT().UpdateLastLogin(ctx, userID).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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

func TestLogin_EmailNotVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)
	user.EmailVerified = false

	userRepo.EXPECT().GetUserByEmail(ctx, "test@example.com").Return(user, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	_, err := svc.Login(ctx, LoginParams{
		Email:    "test@example.com",
		Password: "password",
	})

	if !errors.Is(err, domain.ErrEmailNotVerified) {
		t.Errorf("expected ErrEmailNotVerified, got %v", err)
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

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)
	user.IsBanned = true

	userRepo.EXPECT().GetUserByEmail(ctx, gomock.Any()).Return(user, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)

	userRepo.EXPECT().GetUserByEmail(ctx, gomock.Any()).Return(user, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)
	user.EmailVerified = true
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	accessPayload := makePayload(userID, token.TokenTypeAccess)
	rt := makeRefreshToken(userID)

	maker.EXPECT().VerifyToken("refresh-token", token.TokenTypeRefresh).Return(refreshPayload, nil)
	rtRepo.EXPECT().GetRefreshTokenByHash(ctx, gomock.Any()).Return(rt, nil)
	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	maker.EXPECT().CreateToken(userID, "testuser", "user", false, testAccessDur, token.TokenTypeAccess).Return("new-access-token", accessPayload, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	rt := makeRefreshToken(userID)
	rt.RevokedAt = sql.NullTime{Time: time.Now(), Valid: true}

	maker.EXPECT().VerifyToken("refresh-token", token.TokenTypeRefresh).Return(refreshPayload, nil)
	rtRepo.EXPECT().GetRefreshTokenByHash(ctx, gomock.Any()).Return(rt, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	_, err := svc.RefreshToken(ctx, "bad-token")

	if !errors.Is(err, token.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefreshToken_BannedUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)
	user.IsBanned = true
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)
	rt := makeRefreshToken(userID)

	maker.EXPECT().VerifyToken("refresh-token", token.TokenTypeRefresh).Return(refreshPayload, nil)
	rtRepo.EXPECT().GetRefreshTokenByHash(ctx, gomock.Any()).Return(rt, nil)
	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	_, err := svc.RefreshToken(ctx, "refresh-token")

	if !errors.Is(err, domain.ErrUserBanned) {
		t.Errorf("expected ErrUserBanned, got %v", err)
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
	ctx := context.Background()

	userID := uuid.New()
	refreshPayload := makePayload(userID, token.TokenTypeRefresh)

	maker.EXPECT().VerifyToken("refresh-token", token.TokenTypeRefresh).Return(refreshPayload, nil)
	rtRepo.EXPECT().RevokeByHash(ctx, gomock.Any()).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.Logout(ctx, "bad-token")

	if !errors.Is(err, token.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
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
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		TokenHash: "somehash",
		Type:      domain.VerificationTypeEmail,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)
	vtRepo.EXPECT().MarkUsed(ctx, gomock.Any()).Return(nil)
	userRepo.EXPECT().MarkEmailVerified(ctx, userID).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypeEmail,
		ExpiresAt: time.Now().Add(time.Hour),
		UsedAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypeEmail,
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.VerifyEmail(ctx, "expired-token")

	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyEmail_WrongTokenType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypePasswordReset, // wrong type
		ExpiresAt: time.Now().Add(time.Hour),
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.VerifyEmail(ctx, "rawtoken")

	if !errors.Is(err, domain.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
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

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)

	userRepo.EXPECT().GetUserByEmail(ctx, "test@example.com").Return(user, nil)
	vtRepo.EXPECT().CreateVerificationToken(ctx, userID, gomock.Any(), domain.VerificationTypePasswordReset, gomock.Any()).Return(db.VerificationToken{}, nil)
	pub.EXPECT().PublishPasswordResetRequested(ctx, gomock.Any()).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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

	userRepo.EXPECT().GetUserByEmail(ctx, "unknown@example.com").Return(db.User{}, domain.ErrUserNotFound)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypePasswordReset,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)
	vtRepo.EXPECT().MarkUsed(ctx, gomock.Any()).Return(nil)
	userRepo.EXPECT().UpdatePassword(ctx, gomock.Any()).Return(nil)
	rtRepo.EXPECT().RevokeAllForUser(ctx, userID).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
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
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypeEmail, // wrong type
		ExpiresAt: time.Now().Add(time.Hour),
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.ResetPassword(ctx, "rawtoken", "newpassword123")

	if !errors.Is(err, domain.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypePasswordReset,
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.ResetPassword(ctx, "rawtoken", "newpassword123")

	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestResetPassword_AlreadyUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	vt := db.VerificationToken{
		TokenID:   uuid.New(),
		UserID:    userID,
		Type:      domain.VerificationTypePasswordReset,
		ExpiresAt: time.Now().Add(time.Hour),
		UsedAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}

	vtRepo.EXPECT().GetByHash(ctx, gomock.Any()).Return(vt, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.ResetPassword(ctx, "rawtoken", "newpassword123")

	if !errors.Is(err, domain.ErrTokenAlreadyUsed) {
		t.Errorf("expected ErrTokenAlreadyUsed, got %v", err)
	}
}

// ------- SendEmailVerification -------

func TestSendEmailVerification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	vtRepo.EXPECT().CreateVerificationToken(ctx, userID, gomock.Any(), domain.VerificationTypeEmail, gomock.Any()).Return(db.VerificationToken{}, nil)
	pub.EXPECT().PublishEmailVerificationRequested(ctx, gomock.Any()).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.SendEmailVerification(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendEmailVerification_AlreadyVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)
	user.EmailVerified = true

	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.SendEmailVerification(ctx, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendEmailVerification_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	userRepo.EXPECT().GetUserByID(ctx, userID).Return(db.User{}, domain.ErrUserNotFound)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.SendEmailVerification(ctx, userID)

	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

// ------- ResendEmailVerification -------

func TestResendEmailVerification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	user := makeUser(userID)

	userRepo.EXPECT().GetUserByEmail(ctx, "test@example.com").Return(user, nil)
	userRepo.EXPECT().GetUserByID(ctx, userID).Return(user, nil)
	vtRepo.EXPECT().CreateVerificationToken(ctx, userID, gomock.Any(), domain.VerificationTypeEmail, gomock.Any()).Return(db.VerificationToken{}, nil)
	pub.EXPECT().PublishEmailVerificationRequested(ctx, gomock.Any()).Return(nil)

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.ResendEmailVerification(ctx, "test@example.com")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestResendEmailVerification_UnknownEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepo := mock.NewMockUserRepository(ctrl)
	rtRepo := mock.NewMockRefreshTokenRepository(ctrl)
	vtRepo := mock.NewMockVerificationTokenRepository(ctrl)
	maker := mock.NewMockMaker(ctrl)
	pub := mock.NewMockEventPublisher(ctrl)
	ctx := context.Background()

	userRepo.EXPECT().GetUserByEmail(ctx, "unknown@example.com").Return(db.User{}, errors.New("not found"))

	svc := NewAuthService(userRepo, rtRepo, vtRepo, maker, pub, testAccessDur, testRefreshDur, testVerifyTokenDur, testResetTokenDur)
	err := svc.ResendEmailVerification(ctx, "unknown@example.com")

	if err != nil {
		t.Fatalf("expected nil (swallowed), got %v", err)
	}
}
