package token

import (
	"fmt"
	"time"

	"github.com/aead/chacha20poly1305"
	"github.com/google/uuid"
	"github.com/o1egl/paseto"
)

type PasetoMaker struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

func NewPasetoMaker(symmetricKey string) (Maker, error) {
	if len(symmetricKey) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key size: must be exactly %d characters", chacha20poly1305.KeySize)
	}

	return &PasetoMaker{
		paseto:       paseto.NewV2(),
		symmetricKey: []byte(symmetricKey),
	}, nil
}

func (maker *PasetoMaker) CreateToken(userID uuid.UUID, username string, role string, duration time.Duration, tokenType TokenType) (string, *Payload, error) {
	payload, err := NewPayload(userID, username, role, duration, tokenType)
	if err != nil {
		return "", nil, err
	}
	tok, err := maker.paseto.Encrypt(maker.symmetricKey, payload, nil)
	return tok, payload, err
}

func (maker *PasetoMaker) VerifyToken(tok string, tokenType TokenType) (*Payload, error) {
	payload := &Payload{}
	if err := maker.paseto.Decrypt(tok, maker.symmetricKey, payload, nil); err != nil {
		return nil, ErrInvalidToken
	}
	if err := payload.Valid(tokenType); err != nil {
		return nil, err
	}
	return payload, nil
}
