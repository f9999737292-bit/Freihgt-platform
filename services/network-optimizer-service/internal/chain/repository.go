package chain

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

var ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")

// Repository is the persistence port. NLO-0.9C-I0 supplies an in-memory double only.
type Repository interface {
	Claim(ctx context.Context, tenant uuid.UUID, key, hash string) (ClaimResult, error)
	Commit(ctx context.Context, tenant uuid.UUID, key, hash string, result Result) error
}

type ClaimResult struct {
	Replay   bool
	Conflict bool
	Result   Result
}

// MemoryRepository stores search runs for tests. It does not open a database.
type MemoryRepository struct {
	mu      sync.Mutex
	entries []memoryEntry
}

type memoryEntry struct {
	Tenant uuid.UUID
	Key    string
	Hash   string
	Result Result
}

func (m *MemoryRepository) Claim(_ context.Context, tenant uuid.UUID, key, hash string) (ClaimResult, error) {
	if key == "" {
		return ClaimResult{}, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.entries {
		if entry.Tenant != tenant || entry.Key != key {
			continue
		}
		if entry.Hash != hash {
			return ClaimResult{Conflict: true}, ErrIdempotencyConflict
		}
		return ClaimResult{Replay: true, Result: entry.Result}, nil
	}
	return ClaimResult{}, nil
}

func (m *MemoryRepository) Commit(_ context.Context, tenant uuid.UUID, key, hash string, result Result) error {
	if key == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, entry := range m.entries {
		if entry.Tenant == tenant && entry.Key == key {
			if entry.Hash != hash {
				return ErrIdempotencyConflict
			}
			m.entries[i].Result = result
			return nil
		}
	}
	m.entries = append(m.entries, memoryEntry{Tenant: tenant, Key: key, Hash: hash, Result: result})
	return nil
}

func (m *MemoryRepository) Lookup(tenant uuid.UUID, key string) (Result, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.entries {
		if entry.Tenant == tenant && entry.Key == key {
			return entry.Result, true
		}
	}
	return Result{}, false
}
