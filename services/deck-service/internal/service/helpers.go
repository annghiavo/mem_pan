package service

import (
	"database/sql"

	"mem_pan/services/deck-service/internal/db"
)

func nullStr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullLang(s *string) db.NullCardLanguage {
	if s == nil {
		return db.NullCardLanguage{}
	}
	return db.NullCardLanguage{CardLanguage: db.CardLanguage(*s), Valid: true}
}
