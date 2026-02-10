package main

import (
	"sync"
	"time"
)

// bucket holds per-key token state.
type bucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// RateLimiter implements per-key token bucket rate limiting.
// The configured RPS value is both the sustained rate and the burst size.
type RateLimiter struct {
	rps      int
	maxBurst int
	buckets  sync.Map // string -> *bucket
}

// NewRateLimiter creates a per-key rate limiter where rps is the sustained
// requests-per-second rate and also the burst size (1 second of tokens).
func NewRateLimiter(rps int) *RateLimiter {
	return &RateLimiter{
		rps:      rps,
		maxBurst: rps,
	}
}

// Allow checks if a request from the given key should be allowed.
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	val, _ := rl.buckets.LoadOrStore(key, &bucket{
		tokens:     float64(rl.maxBurst),
		lastRefill: now,
	})
	b := val.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	elapsed := now.Sub(b.lastRefill)
	if elapsed > 0 {
		tokensToAdd := elapsed.Seconds() * float64(rl.rps)
		b.tokens = min(float64(rl.maxBurst), b.tokens+tokensToAdd)
		b.lastRefill = now
	}

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}

// Cleanup removes stale buckets that haven't been used in 5 minutes.
func (rl *RateLimiter) Cleanup() {
	threshold := time.Now().Add(-5 * time.Minute)
	rl.buckets.Range(func(key, value any) bool {
		b := value.(*bucket)
		b.mu.Lock()
		stale := b.lastRefill.Before(threshold)
		b.mu.Unlock()
		if stale {
			rl.buckets.Delete(key)
		}
		return true
	})
}
