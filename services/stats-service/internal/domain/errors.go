package domain

import "errors"

var (
	ErrUserStatsNotFound = errors.New("user stats not found")
	ErrDeckStatsNotFound = errors.New("deck stats not found")
	ErrForbidden         = errors.New("forbidden")
)
