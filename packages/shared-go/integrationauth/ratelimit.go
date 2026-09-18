package integrationauth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// PrincipalRateLimiter provides deterministic in-memory per-key rate limiting for tests and gateway.
type PrincipalRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string][]time.Time
	now     func() time.Time
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

// OAuthFailedAttemptKey builds a rate-limit key for unsuccessful token requests.
// The client secret is never included; client ID is hashed to avoid enumeration oracles.
func OAuthFailedAttemptKey(clientIP, clientID string) string {
	normalizedID := strings.ToLower(strings.TrimSpace(clientID))
	sum := sha256.Sum256([]byte(normalizedID))
	return fmt.Sprintf("oauth_fail:%s:%s", strings.TrimSpace(clientIP), hex.EncodeToString(sum[:8]))
}

func (l *PrincipalRateLimiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	entries, now := l.compact(key)
	if len(entries) >= l.limit {
		retryAfter := entries[0].Add(l.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		l.buckets[key] = entries
		return false, retryAfter
	}
	entries = append(entries, now)
	l.buckets[key] = entries
	return true, 0
}

// IsLimited reports whether the key is currently over the configured limit.
func (l *PrincipalRateLimiter) IsLimited(key string) (limited bool, retryAfter time.Duration) {
	entries, now := l.compact(key)
	l.buckets[key] = entries
	if len(entries) >= l.limit {
		retryAfter := entries[0].Add(l.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return true, retryAfter
	}
	return false, 0
}

// RecordAttempt records one rate-limit attempt for the key.
func (l *PrincipalRateLimiter) RecordAttempt(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entries, now := l.compactLocked(key)
	entries = append(entries, now)
	l.buckets[key] = entries
}

func (l *PrincipalRateLimiter) compact(key string) ([]time.Time, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.compactLocked(key)
}

func (l *PrincipalRateLimiter) compactLocked(key string) ([]time.Time, time.Time) {
	now := l.now().UTC()
	cutoff := now.Add(-l.window)
	entries := l.buckets[key]
	filtered := entries[:0]
	for _, ts := range entries {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	return filtered, now
}
