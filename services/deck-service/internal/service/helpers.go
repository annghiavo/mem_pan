package service

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"

	"mem_pan/services/deck-service/internal/db"
	"mem_pan/services/deck-service/internal/domain"
)

func nullStr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullStrVal(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

func nullLang(s *string) db.NullCardLanguage {
	if s == nil {
		return db.NullCardLanguage{}
	}
	return db.NullCardLanguage{CardLanguage: db.CardLanguage(*s), Valid: true}
}

func accessLevelFilter(accessLevel string) sql.NullString {
	if accessLevel == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: accessLevel, Valid: true}
}

func parseDeckAccessLevel(accessLevel string) (db.DeckAccessLevel, error) {
	switch db.DeckAccessLevel(accessLevel) {
	case db.DeckAccessLevelFree, db.DeckAccessLevelPlus, db.DeckAccessLevelPrivate:
		return db.DeckAccessLevel(accessLevel), nil
	default:
		return "", domain.ErrInvalidAccessLevel
	}
}

func parseDeckPlusStatus(plusStatus string) (db.DeckPlusStatus, error) {
	switch db.DeckPlusStatus(plusStatus) {
	case db.DeckPlusStatusSubmitted, db.DeckPlusStatusApproved, db.DeckPlusStatusRejected, db.DeckPlusStatusSuspended:
		return db.DeckPlusStatus(plusStatus), nil
	default:
		return "", domain.ErrInvalidPlusStatus
	}
}

func privilegedRole(role string) bool {
	switch strings.ToLower(role) {
	case "admin", "moderator":
		return true
	default:
		return false
	}
}

func firstRole(roles []string) string {
	if len(roles) == 0 {
		return ""
	}
	return roles[0]
}

func requireFullDeckAccess(ctx context.Context, deck db.Deck, userID uuid.UUID, role string, isPlus bool) error {
	if deck.Status != "" && deck.Status != string(db.ContentStatusActive) && deck.Status != string(db.ContentStatusHidden) {
		return domain.ErrDeckNotFound
	}
	if deck.UserID == userID || privilegedRole(role) {
		return nil
	}
	if !deck.IsPublic || deck.AccessLevel == db.DeckAccessLevelPrivate {
		return domain.ErrForbidden
	}
	if deck.AccessLevel != db.DeckAccessLevelPlus {
		return nil
	}
	if deck.PlusStatus != db.DeckPlusStatusApproved {
		return domain.ErrForbidden
	}
	if !isPlus {
		return domain.ErrPlusRequired
	}
	return nil
}
