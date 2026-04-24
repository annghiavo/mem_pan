package domain

import "errors"

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrEmailNotVerified      = errors.New("email not verified")
	ErrUserBanned            = errors.New("user is banned")
	ErrTokenNotFound         = errors.New("token not found")
	ErrTokenExpired          = errors.New("token has expired")
	ErrTokenAlreadyUsed      = errors.New("token has already been used")
	ErrTokenRevoked          = errors.New("token has been revoked")
)
