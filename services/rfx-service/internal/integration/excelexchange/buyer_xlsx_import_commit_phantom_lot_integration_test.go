//go:build integration

package excelexchange

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestCommitPhantomLotInsertBlockedWhileEventLockedExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)
	_ = preview

	ctx := context.Background()
	holdTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin hold tx: %v", err)
	}
	defer func() { _ = holdTx.Rollback(ctx) }()

	qRepo := env.qRepo.WithTx(holdTx)
	rfxRepo := env.rfxRepo.WithTx(holdTx)
	if _, err := qRepo.LockEventVersionState(ctx, draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("lock event: %v", err)
	}
	if _, err := rfxRepo.LockEventLotsForUpdate(ctx, draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("lock lots: %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		_, createErr := env.rfxSvc.CreateLot(ctx, fix.BuyerA, draft.Event.ID, domain.CreateRfxLotInput{
			TenantID: fix.TenantID, RfxEventID: draft.Event.ID, LotNumber: "L-PHANTOM", Name: "phantom lot",
		})
		done <- createErr
	}()
	<-started
	waitForLockWaiters(t, env, 1)

	select {
	case createErr := <-done:
		t.Fatalf("CreateLot finished before barrier release: %v", createErr)
	default:
	}

	if err := holdTx.Commit(ctx); err != nil {
		t.Fatalf("release event lock: %v", err)
	}

	select {
	case createErr := <-done:
		if createErr != nil {
			t.Fatalf("CreateLot after release: %v", createErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CreateLot did not complete after barrier release")
	}
}

func TestCommitPhantomLotInsertBeforeCommitCausesStaleExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)
	preview := previewReadyAnalysis(t, env, fix, draft)

	if _, err := env.rfxSvc.CreateLot(context.Background(), fix.BuyerA, draft.Event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: draft.Event.ID, LotNumber: "L-PHANTOM", Name: "phantom lot",
	}); err != nil {
		t.Fatalf("insert lot before commit: %v", err)
	}
	before := captureGraphWriteSnapshot(t, env, fix.TenantID, draft.Event.ID)

	rec := postBuyerXlsxImportCommitHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, draft.Event.ID, *preview.AnalysisID, "phantom-stale-b")
	assertHTTPErrorCode(t, rec, http.StatusConflict, apperrors.CodeConflict)
	assertCommitFailureNoWrites(t, env, fix, draft, *preview.AnalysisID, before)
}

func TestCommitPhantomLotUpdateBlockedWhileEventLockedExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	ctx := context.Background()
	holdTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin hold tx: %v", err)
	}
	defer func() { _ = holdTx.Rollback(ctx) }()

	qRepo := env.qRepo.WithTx(holdTx)
	if _, err := qRepo.LockEventVersionState(ctx, draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("lock event: %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		tx, beginErr := env.pool.Begin(ctx)
		if beginErr != nil {
			done <- beginErr
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()
		repo := repository.NewRfxRepository(env.pool).WithTx(tx)
		if err := repo.LockRfxEventForLotMutation(ctx, draft.Event.ID, fix.TenantID); err != nil {
			done <- err
			return
		}
		_, updateErr := repo.UpdateLotByEventAndNumber(ctx, draft.Event.ID, fix.TenantID, draft.Lot.LotNumber, domain.CreateRfxLotInput{
			TenantID: fix.TenantID, RfxEventID: draft.Event.ID, LotNumber: draft.Lot.LotNumber, Name: "blocked rename",
		}, "ACTIVE")
		if updateErr != nil {
			done <- updateErr
			return
		}
		done <- tx.Commit(ctx)
	}()
	<-started
	waitForLockWaiters(t, env, 1)

	select {
	case updateErr := <-done:
		t.Fatalf("lot update finished before barrier release: %v", updateErr)
	default:
	}

	if err := holdTx.Commit(ctx); err != nil {
		t.Fatalf("release event lock: %v", err)
	}

	select {
	case updateErr := <-done:
		if updateErr != nil {
			t.Fatalf("lot update after release: %v", updateErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lot update did not complete after barrier release")
	}
}

func TestCommitPhantomLotSoftDeleteBlockedWhileEventLockedExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	ctx := context.Background()
	holdTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin hold tx: %v", err)
	}
	defer func() { _ = holdTx.Rollback(ctx) }()

	qRepo := env.qRepo.WithTx(holdTx)
	if _, err := qRepo.LockEventVersionState(ctx, draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("lock event: %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		tx, beginErr := env.pool.Begin(ctx)
		if beginErr != nil {
			done <- beginErr
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()
		repo := repository.NewRfxRepository(env.pool).WithTx(tx)
		if err := repo.LockRfxEventForLotMutation(ctx, draft.Event.ID, fix.TenantID); err != nil {
			done <- err
			return
		}
		if err := repo.SoftDeleteLotByEventAndNumber(ctx, draft.Event.ID, fix.TenantID, draft.Lot.LotNumber); err != nil {
			done <- err
			return
		}
		done <- tx.Commit(ctx)
	}()
	<-started
	waitForLockWaiters(t, env, 1)

	select {
	case deleteErr := <-done:
		t.Fatalf("lot soft-delete finished before barrier release: %v", deleteErr)
	default:
	}

	if err := holdTx.Commit(ctx); err != nil {
		t.Fatalf("release event lock: %v", err)
	}

	select {
	case deleteErr := <-done:
		if deleteErr != nil {
			t.Fatalf("lot soft-delete after release: %v", deleteErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lot soft-delete did not complete after barrier release")
	}
}

func TestCommitPhantomTwoLotInsertsRespectBarrierAndUniqueExtra(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	draft := seedRichDraftEvent(t, env, fix)

	ctx := context.Background()
	holdTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin hold tx: %v", err)
	}
	defer func() { _ = holdTx.Rollback(ctx) }()

	qRepo := env.qRepo.WithTx(holdTx)
	if _, err := qRepo.LockEventVersionState(ctx, draft.Event.ID, fix.TenantID); err != nil {
		t.Fatalf("lock event: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for i, lotNumber := range []string{"L-NEW-A", "L-NEW-B"} {
		wg.Add(1)
		lotNumber := lotNumber
		go func(idx int) {
			defer wg.Done()
			_, createErr := env.rfxSvc.CreateLot(ctx, fix.BuyerA, draft.Event.ID, domain.CreateRfxLotInput{
				TenantID: fix.TenantID, RfxEventID: draft.Event.ID, LotNumber: lotNumber, Name: "new lot",
			})
			errCh <- createErr
		}(i)
	}
	waitForLockWaiters(t, env, 1)

	select {
	case createErr := <-errCh:
		t.Fatalf("insert completed before barrier release: %v", createErr)
	default:
	}

	if err := holdTx.Commit(ctx); err != nil {
		t.Fatalf("release event lock: %v", err)
	}

	wg.Wait()
	close(errCh)
	for createErr := range errCh {
		if createErr != nil {
			t.Fatalf("insert after release: %v", createErr)
		}
	}

	_, dupErr := env.rfxSvc.CreateLot(ctx, fix.BuyerA, draft.Event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: draft.Event.ID, LotNumber: "L-NEW-A", Name: "duplicate",
	})
	if dupErr == nil {
		t.Fatal("expected duplicate lot_number insert to fail")
	}
	var appErr *apperrors.AppError
	if !errors.As(dupErr, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict on duplicate lot_number, got %v", dupErr)
	}
}
