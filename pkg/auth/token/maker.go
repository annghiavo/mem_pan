package token

import (
	"time"

	"github.com/google/uuid"
)

type Maker interface {
	CreateToken(userID uuid.UUID, username string, role string, duration time.Duration, tokenType TokenType) (string, *Payload, error)
	VerifyToken(token string, tokenType TokenType) (*Payload, error)
}