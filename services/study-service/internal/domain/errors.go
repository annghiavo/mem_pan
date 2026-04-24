package domain

import "errors"

var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionFinished  = errors.New("session is already finished")
	ErrCardNotInSession = errors.New("card not in session")
	ErrCardAlreadyReviewed = errors.New("card already reviewed in this session")
	ErrForbidden        = errors.New("access denied")
	ErrInvalidRating    = errors.New("rating must be between 1 and 4")
	ErrDeckEmpty        = errors.New("no cards available to study in this deck")
)
