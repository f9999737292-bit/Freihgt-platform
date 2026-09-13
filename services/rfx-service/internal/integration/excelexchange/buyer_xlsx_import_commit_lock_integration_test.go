//go:build integration

package excelexchange

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestCommitImportAnalysisForUpdateSerializationExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	if preview.AnalysisID == nil {
		t.Fatal("expected analysis_id from preview")
	}
	analysisID := *preview.AnalysisID

	ctx := context.Background()
	holdTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin hold tx: %v", err)
	}
	t.Cleanup(func() { _ = holdTx.Rollback(ctx) })

	lockedRepo := env.importAnalysisRepo.WithTx(holdTx)
	if _, err := lockedRepo.LockImportAnalysisForUpdate(ctx, analysisID, fix.TenantID); err != nil {
		t.Fatalf("lock analysis for update: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	errCh := make(chan error, 1)
	go func() {
		defer wg.Done()
		tx, beginErr := env.pool.Begin(ctx)
		if beginErr != nil {
			errCh <- beginErr
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()
		repo := env.importAnalysisRepo.WithTx(tx)
		if _, lockErr := repo.LockImportAnalysisForUpdate(ctx, analysisID, fix.TenantID); lockErr != nil {
			errCh <- lockErr
			return
		}
		errCh <- tx.Commit(ctx)
	}()

	waitForLockWaiters(t, env, 1)
	if err := holdTx.Commit(ctx); err != nil {
		t.Fatalf("release analysis lock: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("second locker failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second analysis lock")
	}
	wg.Wait()
}

func TestCommitEventLotsForUpdateSerializationExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	ctx := context.Background()
	holdTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin hold tx: %v", err)
	}
	t.Cleanup(func() { _ = holdTx.Rollback(ctx) })

	lockedRepo := env.rfxRepo.WithTx(holdTx)
	if _, err := lockedRepo.LockEventLotsForUpdate(ctx, draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("lock event lots for update: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	errCh := make(chan error, 1)
	go func() {
		defer wg.Done()
		tx, beginErr := env.pool.Begin(ctx)
		if beginErr != nil {
			errCh <- beginErr
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()
		repo := repository.NewRfxRepository(env.pool).WithTx(tx)
		if _, lockErr := repo.LockEventLotsForUpdate(ctx, draft.Event.ID, fix.TenantID); lockErr != nil {
			errCh <- lockErr
			return
		}
		errCh <- tx.Commit(ctx)
	}()

	waitForLockWaiters(t, env, 1)
	if err := holdTx.Commit(ctx); err != nil {
		t.Fatalf("release lot lock: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("second lot locker failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second lot lock")
	}
	wg.Wait()
}

func waitForLockWaiters(t *testing.T, env *testEnv, minWaiters int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := env.pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock'
			  AND state = 'active'
			  AND pid <> pg_backend_pid()`).Scan(&waiting)
		if err == nil && waiting >= minWaiters {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d lock waiters", minWaiters)
}
