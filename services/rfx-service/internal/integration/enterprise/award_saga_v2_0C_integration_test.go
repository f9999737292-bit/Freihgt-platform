//go:build integration

package enterprise

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/transportorderclient"
)

func TestCAward001HappyPathCreatesTOSnapshotAndLink(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 125000, 124000)

	result, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if result == nil || !result.Created || len(result.Items) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	var orderCount, snapshotCount, linkCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`, fix.TenantID).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transport.transport_order_rate_snapshots s
		JOIN transport.transport_orders o ON o.id = s.transport_order_id
		WHERE s.tenant_id = $1 AND o.pricing_model_version = 'SNAPSHOT_V1'`, fix.TenantID).Scan(&snapshotCount); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_award_transport_orders WHERE tenant_id = $1 AND rfx_event_id = $2`,
		fix.TenantID, event.ID).Scan(&linkCount); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if orderCount != 2 || snapshotCount != 2 || linkCount != 2 {
		t.Fatalf("orders=%d snapshots=%d links=%d", orderCount, snapshotCount, linkCount)
	}
}

func TestCAward002IdempotentRetryReusesTOSnapshot(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 100000, 95000)

	first, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil || !first.Created {
		t.Fatalf("first convert: err=%v created=%v", err, first.Created)
	}
	second, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil || second.Created {
		t.Fatalf("second convert: err=%v created=%v", err, second.Created)
	}
	for i := range first.Items {
		if first.Items[i].TransportOrderID != second.Items[i].TransportOrderID {
			t.Fatalf("transport order id changed on idempotent retry")
		}
	}
	var idempotencyCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_order_create_idempotency WHERE tenant_id = $1`, fix.TenantID).Scan(&idempotencyCount); err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if idempotencyCount != 2 {
		t.Fatalf("expected 2 idempotency rows, got %d", idempotencyCount)
	}
}

func TestCAward003LinkFailureThenRetryCompletes(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 88000, 77000)

	env.auditRepo.SetInjectRecordFailure(true)
	_, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	env.auditRepo.SetInjectRecordFailure(false)
	if err == nil {
		t.Fatal("expected injected link-phase failure")
	}
	var orderCountBeforeRetry int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`, fix.TenantID).Scan(&orderCountBeforeRetry); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCountBeforeRetry != 2 {
		t.Fatalf("TO+snapshot should be committed before link failure, got orders=%d", orderCountBeforeRetry)
	}
	var linkCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_award_transport_orders WHERE tenant_id = $1 AND rfx_event_id = $2`,
		fix.TenantID, event.ID).Scan(&linkCount); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if linkCount != 0 {
		t.Fatalf("expected no award links after failed link phase, got %d", linkCount)
	}

	result, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("retry convert: %v", err)
	}
	if !result.Created {
		t.Fatal("retry should create award links")
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_award_transport_orders WHERE tenant_id = $1 AND rfx_event_id = $2`,
		fix.TenantID, event.ID).Scan(&linkCount); err != nil {
		t.Fatalf("count links after retry: %v", err)
	}
	if linkCount != 2 {
		t.Fatalf("expected 2 links after retry, got %d", linkCount)
	}
}

func TestCAward004LinkFailureNoDuplicateTO(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 111000, 222000)

	firstTOIDs := make(map[uuid.UUID]struct{})
	env.auditRepo.SetInjectRecordFailure(true)
	if _, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID); err == nil {
		t.Fatal("expected link failure on first attempt")
	}
	env.auditRepo.SetInjectRecordFailure(false)

	rows, err := env.pool.Query(ctx, `SELECT id FROM transport.transport_orders WHERE tenant_id = $1 ORDER BY created_at`, fix.TenantID)
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan order: %v", err)
		}
		firstTOIDs[id] = struct{}{}
	}
	rows.Close()

	result, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	for _, item := range result.Items {
		if _, ok := firstTOIDs[item.TransportOrderID]; !ok {
			t.Fatalf("retry created new transport order %s", item.TransportOrderID)
		}
	}
	var orderCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`, fix.TenantID).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 2 {
		t.Fatalf("expected exactly 2 orders after failure+retry, got %d", orderCount)
	}
}

func TestCAward005MultiLotSnapshotAmountsPreserved(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, lotA, lotB := seedAwardedMultiLotEvent(t, env, fix, 111111, 222222)

	result, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	amounts := map[uuid.UUID]float64{}
	for _, item := range result.Items {
		amounts[item.RfxLotID] = item.Amount
	}
	if amounts[lotA] != 111111 || amounts[lotB] != 222222 {
		t.Fatalf("award amounts not preserved: lotA=%v lotB=%v", amounts[lotA], amounts[lotB])
	}
	for _, item := range result.Items {
		var totalText string
		err := env.pool.QueryRow(ctx, `
			SELECT total_amount::text FROM transport.transport_order_rate_snapshots
			WHERE transport_order_id = $1 AND tenant_id = $2`, item.TransportOrderID, fix.TenantID).Scan(&totalText)
		if err != nil {
			t.Fatalf("snapshot total: %v", err)
		}
		want := fmt.Sprintf("%.2f", item.Amount)
		if totalText != want {
			t.Fatalf("snapshot total=%s want %s for lot %s", totalText, want, item.RfxLotID)
		}
	}
}

func TestCAward006ConcurrentConversionNoDuplicate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 120000, 80000)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
		}(i)
	}
	wg.Wait()
	success := 0
	for _, err := range errs {
		if err == nil {
			success++
		}
	}
	if success == 0 {
		t.Fatalf("both conversions failed: %v %v", errs[0], errs[1])
	}
	var orderCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`, fix.TenantID).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 2 {
		t.Fatalf("expected exactly 2 orders after concurrent conversion, got %d", orderCount)
	}
}

func TestCAward007IdempotencyKeyStablePerLot(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, lotA, lotB := seedAwardedMultiLotEvent(t, env, fix, 50000, 60000)

	if _, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("convert: %v", err)
	}
	keys := map[string]uuid.UUID{}
	rows, err := env.pool.Query(ctx, `
		SELECT idempotency_key, transport_order_id
		FROM transport.transport_order_create_idempotency
		WHERE tenant_id = $1`, fix.TenantID)
	if err != nil {
		t.Fatalf("list idempotency: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var orderID uuid.UUID
		if err := rows.Scan(&key, &orderID); err != nil {
			t.Fatalf("scan idempotency: %v", err)
		}
		keys[key] = orderID
	}
	wantA := transportorderclient.AwardConversionIdempotencyKey(fix.TenantID, event.ID, lotA)
	wantB := transportorderclient.AwardConversionIdempotencyKey(fix.TenantID, event.ID, lotB)
	if _, ok := keys[wantA]; !ok {
		t.Fatalf("missing idempotency key for lot A: %s", wantA)
	}
	if _, ok := keys[wantB]; !ok {
		t.Fatalf("missing idempotency key for lot B: %s", wantB)
	}
	if keys[wantA] == keys[wantB] {
		t.Fatal("lot idempotency keys must map to distinct transport orders")
	}
}
