package integrationauth

import (
	"fmt"
	"sync"
	"time"
)

// PrincipalRateLimiter provides deterministic in-memory per-key rate limiting for tests and gateway.
type PrincipalRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	buckets  map[string][]time.Time
	now      func() time.Time
}

func NewPrincipalRateLimiter(limit int, window time.Duration) *PrincipalRateLimiter {
	if limit <= 0 {
		limit = 30
	}
	if window <= 0 {
		window = time.Minute
	}
	return &PrincipalRateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string][]time.Time),
		now:     time.Now,
	}
}

func (l *PrincipalRateLimiter) Key(tenantID, principalID, operation string) string {
	return fmt.Sprintf("%s:%s:%s", tenantID, principalID, operation)
}

func (l *PrincipalRateLimiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	cutoff := now.Add(-l.window)
	entries := l.buckets[key]
	filtered := entries[:0]
	for _, ts := range entries {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	if len(filtered) >= l.limit {
		retryAfter = filtered[0].Add(l.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		l.buckets[key] = filtered
		return false, retryAfter
	}
	filtered = append(filtered, now)
	l.buckets[key] = filtered
	return true, 0
}
