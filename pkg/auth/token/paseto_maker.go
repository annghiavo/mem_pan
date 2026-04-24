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

	maker := &PasetoMaker{
		paseto:       paseto.NewV2(),
		symmetricKey: []byte(symmetricKey),
	}

	return maker, nil
}

func (maker *PasetoMaker) CreateToken(userID uuid.UUID, username string, role string, duration time.Duration, tokenType TokenType) (string, *Payload, error) {
	payload, err := NewPayload(userID, username, role, duration, tokenType)
	if err != nil {
		return "", nil, err
	}

	// PASETO V2 Encrypt sử dụng mã hóa đối xứng (Local token)
	// Payload được đưa vào làm JSON claims
	token, err := maker.paseto.Encrypt(maker.symmetricKey, payload, nil)
	return token, payload, err
}

// VerifyToken giải mã và kiểm tra tính hợp lệ của token
func (maker *PasetoMaker) VerifyToken(token string, tokenType TokenType) (*Payload, error) {
	payload := &Payload{}

	// Giải mã token vào struct payload
	err := maker.paseto.Decrypt(token, maker.symmetricKey, payload, nil)
	if err != nil {
		// Paseto Decrypt lỗi thường do Key sai hoặc Token bị giả mạo
		return nil, ErrInvalidToken
	}

	// Gọi hàm Valid(tokenType) để check Expiration và khớp loại Token (Access/Refresh/Verify)
	err = payload.Valid(tokenType)
	if err != nil {
		return nil, err
	}

	return payload, nil
}