package domain

import "errors"

var (
	ErrFolderNotFound   = errors.New("folder not found")
	ErrDeckNotFound     = errors.New("deck not found")
	ErrNoteNotFound     = errors.New("note not found")
	ErrCardNotFound     = errors.New("card not found")
	ErrForbidden        = errors.New("access denied")
	ErrDeckAlreadyInFolder = errors.New("deck already in folder")
	ErrDeckNotInFolder  = errors.New("deck not in folder")
	ErrDeckDeleted      = errors.New("deck has been deleted")
)
