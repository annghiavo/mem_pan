package domain

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"mem_pan/services/auth-service/internal/db"
)

const (
	VerificationTypeEmail         = string(db.VerificationTokenTypeEmailVerification)
	VerificationTypePasswordReset = string(db.VerificationTokenTypePasswordReset)
)

type UserResponse struct {
	UserID        uuid.UUID  `json:"user_id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	FullName      *string    `json:"full_name,omitempty"`
	AvatarURL     *string    `json:"avatar_url,omitempty"`
	Role          string     `json:"role"`
	EmailVerified bool       `json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
}

func UserToResponse(u db.User) UserResponse {
	r := UserResponse{
		UserID:        u.UserID,
		Username:      u.Username,
		Email:         u.Email,
		Role:          u.Role,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}
	if u.FullName.Valid {
		r.FullName = &u.FullName.String
	}
	if u.AvatarUrl.Valid {
		r.AvatarURL = &u.AvatarUrl.String
	}
	return r
}

func NullStr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
