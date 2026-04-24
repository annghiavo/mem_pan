package token

import (
	"time"
)

// Maker is an interface for managing tokens
type Maker interface {
	CreateToken(username string, role string, duration time.Duration, tokenType TokenType) (string, *Payload, error)
	VerifyToken(token string, tokenType TokenType) (*Payload, error)
}