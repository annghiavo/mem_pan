package cache

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	counts map[string]*bucket
	limit  int
	window time.Duration
}

type bucket struct {
	count     int
	windowEnd time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		counts: make(map[string]*bucket),
		limit:  limit,
		window: window,
	}
}

func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	b, ok := r.counts[key]
	if !ok || now.After(b.windowEnd) {
		r.counts[key] = &bucket{count: 1, windowEnd: now.Add(r.window)}
		return true
	}
	if b.count >= r.limit {
		return false
	}
	b.count++
	return true
}

func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for key, b := range r.counts {
		if now.After(b.windowEnd) {
			delete(r.counts, key)
		}
	}
}
