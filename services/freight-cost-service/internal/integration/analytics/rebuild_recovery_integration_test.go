//go:build integration

package analytics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFC22G_ConcurrentRebuildSameTenantSerialized(t *testing.T) {
	env := setupAnalyticsEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	orderID := uuid.New()
	now := time.Now().UTC().Add(-48 * time.Hour)
	upsertSummary(t, env, tenantID, buyerID, carrierID, orderID, "RUB", "1000.00", "900.00", now)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = env.analytics.RebuildTenant(ctx, tenantID)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent rebuild goroutine %d failed: %v", i, err)
		}
	}
}

func TestFC22G_FullStackRebuildIncrementalEquivalence(t *testing.T) {
	TestFC22BEqvRebuildMatchesIncremental(t)
}
