package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"mem_pan/services/auth-service/internal/token"
	"mem_pan/services/auth-service/internal/db"
	"mem_pan/services/auth-service/internal/domain"
	"mem_pan/services/auth-service/internal/publisher"
	"mem_pan/services/auth-service/internal/repository"
)

type AuthTokens struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	TokenID               uuid.UUID
}

type AuthResponse struct {
	Tokens AuthTokens
	User   db.User
}

type RegisterParams struct {
	Username string
	Email    string
	Password string
	FullName *string
}

type LoginParams struct {
	Email     string
	Password  string
	UserAgent string
	ClientIP  string
}

type AuthService interface {
	Register(ctx context.Context, params RegisterParams) (db.User, error)
	Login(ctx context.Context, params LoginParams) (AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (AuthTokens, error)
	Logout(ctx context.Context, refreshToken string) error
	SendEmailVerification(ctx context.Context, userID uuid.UUID) error
	ResendEmailVerification(ctx context.Context, email string) error
	VerifyEmail(ctx context.Context, rawToken string) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, rawToken, newPassword string) error
}

type authService struct {
	userRepo         repository.UserRepository
	refreshTokenRepo repository.RefreshTokenRepository
	verifyTokenRepo  repository.VerificationTokenRepository
	tokenMaker       token.Maker
	publisher        publisher.EventPublisher
	accessDur        time.Duration
	refreshDur       time.Duration
	verifyTokenDur   time.Duration
	resetTokenDur    time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	verifyTokenRepo repository.VerificationTokenRepository,
	tokenMaker token.Maker,
	pub publisher.EventPublisher,
	accessDur, refreshDur, verifyTokenDur, resetTokenDur time.Duration,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		verifyTokenRepo:  verifyTokenRepo,
		tokenMaker:       tokenMaker,
		publisher:        pub,
		accessDur:        accessDur,
		refreshDur:       refreshDur,
		verifyTokenDur:   verifyTokenDur,
		resetTokenDur:    resetTokenDur,
	}
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func extractIP(addr string) *string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		if addr != "" {
			return &addr
		}
		return nil
	}
	if host == "" {
		return nil
	}
	return &host
}

func (s *authService) Register(ctx context.Context, params RegisterParams) (db.User, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return db.User{}, err
	}

	arg := db.CreateUserParams{
		Username:     params.Username,
		Email:        params.Email,
		PasswordHash: string(hashed),
		Role:         "user",
	}
	if params.FullName != nil {
		arg.FullName = domain.NullStr(params.FullName)
	}

	user, err := s.userRepo.CreateUser(ctx, arg)
	if err != nil {
		return db.User{}, err
	}

	_ = s.publisher.PublishUserRegistered(ctx, publisher.UserRegisteredEvent{
		UserID:    user.UserID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	})

	_ = s.SendEmailVerification(ctx, user.UserID)

	return user, nil
}

func (s *authService) Login(ctx context.Context, params LoginParams) (AuthResponse, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, params.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return AuthResponse{}, domain.ErrInvalidCredentials
		}
		return AuthResponse{}, err
	}

	if user.IsBanned {
		return AuthResponse{}, domain.ErrUserBanned
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(params.Password)); err != nil {
		return AuthResponse{}, domain.ErrInvalidCredentials
	}

	accessToken, accessPayload, err := s.tokenMaker.CreateToken(
		user.UserID, user.Username, user.Role, s.accessDur, token.TokenTypeAccess,
	)
	if err != nil {
		return AuthResponse{}, err
	}

	refreshToken, refreshPayload, err := s.tokenMaker.CreateToken(
		user.UserID, user.Username, user.Role, s.refreshDur, token.TokenTypeRefresh,
	)
	if err != nil {
		return AuthResponse{}, err
	}

	_ = s.refreshTokenRepo.DeleteExpiredForUser(ctx, user.UserID)

	var ua *string
	if params.UserAgent != "" {
		ua = &params.UserAgent
	}

	rt, err := s.refreshTokenRepo.CreateRefreshToken(
		ctx, user.UserID, hashToken(refreshToken),
		ua, extractIP(params.ClientIP), refreshPayload.ExpiredAt,
	)
	if err != nil {
		return AuthResponse{}, err
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.UserID)

	return AuthResponse{
		Tokens: AuthTokens{
			AccessToken:           accessToken,
			AccessTokenExpiresAt:  accessPayload.ExpiredAt,
			RefreshToken:          refreshToken,
			RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
			TokenID:               rt.TokenID,
		},
		User: user,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (AuthTokens, error) {
	payload, err := s.tokenMaker.VerifyToken(refreshToken, token.TokenTypeRefresh)
	if err != nil {
		return AuthTokens{}, err
	}

	rt, err := s.refreshTokenRepo.GetRefreshTokenByHash(ctx, hashToken(refreshToken))
	if err != nil {
		return AuthTokens{}, domain.ErrTokenNotFound
	}
	if rt.RevokedAt.Valid {
		return AuthTokens{}, domain.ErrTokenRevoked
	}

	user, err := s.userRepo.GetUserByID(ctx, payload.UserID)
	if err != nil {
		return AuthTokens{}, err
	}
	if user.IsBanned {
		return AuthTokens{}, domain.ErrUserBanned
	}

	accessToken, accessPayload, err := s.tokenMaker.CreateToken(
		user.UserID, user.Username, user.Role, s.accessDur, token.TokenTypeAccess,
	)
	if err != nil {
		return AuthTokens{}, err
	}

	return AuthTokens{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: rt.ExpiresAt,
		TokenID:               rt.TokenID,
	}, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	if _, err := s.tokenMaker.VerifyToken(refreshToken, token.TokenTypeRefresh); err != nil {
		return err
	}
	return s.refreshTokenRepo.RevokeByHash(ctx, hashToken(refreshToken))
}

func (s *authService) SendEmailVerification(ctx context.Context, userID uuid.UUID) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.EmailVerified {
		return nil
	}

	rawToken, err := generateSecureToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(s.verifyTokenDur)
	_, err = s.verifyTokenRepo.CreateVerificationToken(
		ctx, userID, hashToken(rawToken), domain.VerificationTypeEmail, expiresAt,
	)
	if err != nil {
		return err
	}

	_ = s.publisher.PublishEmailVerificationRequested(ctx, publisher.EmailVerificationRequestedEvent{
		UserID:    userID,
		Email:     user.Email,
		Token:     rawToken,
		ExpiresAt: expiresAt,
	})

	return nil
}

func (s *authService) ResendEmailVerification(ctx context.Context, email string) error {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil // swallow to avoid email enumeration
	}
	return s.SendEmailVerification(ctx, user.UserID)
}

func (s *authService) VerifyEmail(ctx context.Context, rawToken string) error {
	vt, err := s.verifyTokenRepo.GetByHash(ctx, hashToken(rawToken))
	if err != nil {
		return domain.ErrTokenNotFound
	}
	if vt.UsedAt.Valid {
		return domain.ErrTokenAlreadyUsed
	}
	if time.Now().After(vt.ExpiresAt) {
		return domain.ErrTokenExpired
	}
	if vt.Type != domain.VerificationTypeEmail {
		return domain.ErrTokenNotFound
	}

	if err := s.verifyTokenRepo.MarkUsed(ctx, hashToken(rawToken)); err != nil {
		return err
	}
	return s.userRepo.MarkEmailVerified(ctx, vt.UserID)
}

func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil // swallow to avoid email enumeration
	}

	rawToken, err := generateSecureToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(s.resetTokenDur)
	_, err = s.verifyTokenRepo.CreateVerificationToken(
		ctx, user.UserID, hashToken(rawToken), domain.VerificationTypePasswordReset, expiresAt,
	)
	if err != nil {
		return err
	}

	_ = s.publisher.PublishPasswordResetRequested(ctx, publisher.PasswordResetRequestedEvent{
		UserID:    user.UserID,
		Email:     user.Email,
		Token:     rawToken,
		ExpiresAt: expiresAt,
	})

	return nil
}

func (s *authService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	vt, err := s.verifyTokenRepo.GetByHash(ctx, hashToken(rawToken))
	if err != nil {
		return domain.ErrTokenNotFound
	}
	if vt.UsedAt.Valid {
		return domain.ErrTokenAlreadyUsed
	}
	if time.Now().After(vt.ExpiresAt) {
		return domain.ErrTokenExpired
	}
	if vt.Type != domain.VerificationTypePasswordReset {
		return domain.ErrTokenNotFound
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.verifyTokenRepo.MarkUsed(ctx, hashToken(rawToken)); err != nil {
		return err
	}
	if err := s.userRepo.UpdatePassword(ctx, db.UpdatePasswordParams{
		UserID:       vt.UserID,
		PasswordHash: string(hashed),
	}); err != nil {
		return err
	}

	_ = s.refreshTokenRepo.RevokeAllForUser(ctx, vt.UserID)

	return nil
}
