package domain

import "errors"

var (
	ErrFolderNotFound         = errors.New("folder not found")
	ErrDeckNotFound           = errors.New("deck not found")
	ErrNoteNotFound           = errors.New("note not found")
	ErrCardNotFound           = errors.New("card not found")
	ErrForbidden              = errors.New("access denied")
	ErrDeckAlreadyInFolder    = errors.New("deck already in folder")
	ErrDeckNotInFolder        = errors.New("deck not in folder")
	ErrDeckDeleted            = errors.New("deck has been deleted")
	ErrCreatorProfileNotFound = errors.New("creator profile not found")
	ErrPlusRequired           = errors.New("active Plus subscription required")
	ErrInvalidAccessLevel     = errors.New("invalid deck access_level")
	ErrInvalidPlusStatus      = errors.New("invalid deck plus_status")
	ErrReviewNotAllowed       = errors.New("review is not allowed")
)
