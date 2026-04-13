package api

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"

	db "mem_pan/db/sqlc"
)

var ErrForbidden = errors.New("forbidden")

func canAccessDeck(userID uuid.UUID, deck db.Deck) bool {
	if userID == deck.UserID {
		return true
	}
	return deck.IsPublic.Valid && deck.IsPublic.Bool &&
		deck.Status.Valid && deck.Status.ContentStatus == db.ContentStatusActive
}

func toNullString(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func toNullBool(v *bool) sql.NullBool {
	if v == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *v, Valid: true}
}

func toNullStatus(v *db.ContentStatus) db.NullContentStatus {
	if v == nil {
		return db.NullContentStatus{}
	}
	return db.NullContentStatus{ContentStatus: *v, Valid: true}
}
