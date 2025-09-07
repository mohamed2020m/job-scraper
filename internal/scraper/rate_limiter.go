package scraper

import (
	"context"
	"sync"
	"time"
)

// RateLimiter manages rate limiting for different sources
type RateLimiter struct {
	limiters map[string]*sourceLimiter
	mu       sync.RWMutex
}

// sourceLimiter handles rate limiting for a specific source
type sourceLimiter struct {
	tokens   chan struct{}
	refill   *time.Ticker
	limit    int
	duration time.Duration
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*sourceLimiter),
	}
}

// Wait waits for permission to make a request to the specified source
func (rl *RateLimiter) Wait(ctx context.Context, source string, requestsPerMinute int) error {
	limiter := rl.getLimiter(source, requestsPerMinute)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-limiter.tokens:
		return nil
	}
}

// getLimiter gets or creates a rate limiter for a source
func (rl *RateLimiter) getLimiter(source string, requestsPerMinute int) *sourceLimiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[source]
	rl.mu.RUnlock()

	if exists && limiter.limit == requestsPerMinute {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := rl.limiters[source]; exists && limiter.limit == requestsPerMinute {
		return limiter
	}

	// Clean up existing limiter if it exists
	if limiter, exists := rl.limiters[source]; exists {
		limiter.refill.Stop()
	}

	// Create new limiter
	duration := time.Minute
	tokens := make(chan struct{}, requestsPerMinute)

	// Fill initial tokens
	for i := 0; i < requestsPerMinute; i++ {
		tokens <- struct{}{}
	}

	refillTicker := time.NewTicker(duration / time.Duration(requestsPerMinute))

	limiter = &sourceLimiter{
		tokens:   tokens,
		refill:   refillTicker,
		limit:    requestsPerMinute,
		duration: duration,
	}

	// Start refill goroutine
	go limiter.startRefill()

	rl.limiters[source] = limiter
	return limiter
}

// startRefill continuously refills tokens
func (sl *sourceLimiter) startRefill() {
	for range sl.refill.C {
		select {
		case sl.tokens <- struct{}{}:
		default:
			// Channel full, skip this refill
		}
	}
}

// Stop stops all rate limiters
func (rl *RateLimiter) Stop() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for _, limiter := range rl.limiters {
		limiter.refill.Stop()
	}
}
