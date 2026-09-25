package routing

import (
	"context"
	"sync"
	"time"
)

const (
	StaticRouteTTL = 24 * time.Hour
	LiveRouteTTL   = 15 * time.Minute
)

type Cache interface {
	Get(ctx context.Context, key string, now time.Time) (RouteResult, bool)
	Put(ctx context.Context, key string, result RouteResult)
}

type MemoryCache struct {
	mu    sync.Mutex
	items map[string]RouteResult
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{items: map[string]RouteResult{}}
}

func (c *MemoryCache) Get(_ context.Context, key string, now time.Time) (RouteResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.items[key]
	if !ok || !result.ExpiresAt.After(now) {
		return RouteResult{}, false
	}
	return result, true
}

func (c *MemoryCache) Put(_ context.Context, key string, result RouteResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = result
}

func Expiry(mode string, calculated time.Time) time.Time {
	if mode == TrafficCurrent {
		return calculated.Add(LiveRouteTTL)
	}
	return calculated.Add(StaticRouteTTL)
}
