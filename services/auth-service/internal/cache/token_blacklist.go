package cache

import (
	"sync"
	"time"
)

type TokenBlacklist struct {
	mu     sync.RWMutex
	tokens map[string]time.Time
}

func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{
		tokens: make(map[string]time.Time),
	}
}

func (b *TokenBlacklist) Add(tokenID string, expiry time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens[tokenID] = expiry
}

func (b *TokenBlacklist) IsBlacklisted(tokenID string) bool {
	b.mu.RLock()
	exp, ok := b.tokens[tokenID]
	b.mu.RUnlock()

	if !ok {
		return false
	}
	if time.Now().After(exp) {
		b.mu.Lock()
		delete(b.tokens, tokenID)
		b.mu.Unlock()
		return false
	}
	return true
}

func (b *TokenBlacklist) Cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	for id, exp := range b.tokens {
		if now.After(exp) {
			delete(b.tokens, id)
		}
	}
}
