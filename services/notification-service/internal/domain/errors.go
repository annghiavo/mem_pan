package domain

import "errors"

var (
	ErrTokenNotFound = errors.New("device token not found")
	ErrForbidden     = errors.New("forbidden")
)
