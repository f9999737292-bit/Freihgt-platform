//go:build integration

package pricingsnapshot

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	todomain "github.com/freight-platform/transport-order-service/internal/domain"
)

func TestCTo001PricedCreateSetsSnapshotV1(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "to-001")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), testRequestHash("to001"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.Order.PricingModelVersion == nil || *result.Order.PricingModelVersion != todomain.PricingModelVersionSnapshotV1 {
		t.Fatalf("expected SNAPSHOT_V1, got %v", result.Order.PricingModelVersion)
	}
}

func TestCTo002PricedCreatePersistsSnapshot(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "to-002")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), testRequestHash("to002"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var count int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_order_rate_snapshots WHERE id = $1`, result.RateSnapshot.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected persisted snapshot row, got count=%d", count)
	}
}

func TestCTo003PricedCreateLinksSnapshotToOrder(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "to-003")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), testRequestHash("to003"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.RateSnapshot.TransportOrderID != result.Order.ID {
		t.Fatalf("snapshot transport_order_id mismatch: snapshot=%s order=%s", result.RateSnapshot.TransportOrderID, result.Order.ID)
	}
}

func TestCTo004PricedCreateIdempotencyRecord(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "to-004")
	hash := testRequestHash("to004")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), hash)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var (
		orderID    uuid.UUID
		snapshotID uuid.UUID
	)
	err = env.pool.QueryRow(ctx, `
		SELECT transport_order_id, rate_snapshot_id
		FROM transport.transport_order_create_idempotency
		WHERE tenant_id = $1 AND actor_company_id = $2 AND idempotency_key = $3`,
		tenantID, buyerID, in.IdempotencyKey).Scan(&orderID, &snapshotID)
	if err != nil {
		t.Fatalf("idempotency row: %v", err)
	}
	if orderID != result.Order.ID || snapshotID != result.RateSnapshot.ID {
		t.Fatal("idempotency record does not point to created commercial result")
	}
}

func TestCTo005PricedCreateRequiresEquipmentDomain(t *testing.T) {
	in := sampleCreateInput(uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), "to-005")
	in.EquipmentType = nil
	if err := todomain.ValidateCreatePricedTransportOrderInput(in); err == nil {
		t.Fatal("expected equipment_type validation error")
	}
}

func TestCTo008PricedCreateStoresResolutionHash(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "to-008")
	snap := sampleSnapshot(tenantID, buyerID, carrierID, originID, destID)
	snap.ResolutionRequestHash = strings.Repeat("h", 64)
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, snap, testRequestHash("to008"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.RateSnapshot.ResolutionRequestHash != strings.Repeat("h", 64) {
		t.Fatalf("unexpected resolution hash: %s", result.RateSnapshot.ResolutionRequestHash)
	}
}

func TestCTo009GetPricedResultReturnsSnapshot(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "to-009")
	created, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), testRequestHash("to009"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reloaded, err := env.pricedOrders.GetPricedResult(ctx, tenantID, created.Order.ID, created.RateSnapshot.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if reloaded.RateSnapshot.ID != created.RateSnapshot.ID {
		t.Fatal("GetPricedResult returned different snapshot")
	}
}

func TestCTo011PricedDraftTransportModeUpdateDenied(t *testing.T) {
	order := &todomain.TransportOrder{PricingModelVersion: ptr("SNAPSHOT_V1")}
	mode := "ROAD"
	err := todomain.ValidateUpdateWithSnapshot(order, todomain.UpdateTransportOrderInput{TransportMode: &mode})
	if err == nil {
		t.Fatal("expected transport_mode update denial")
	}
}

func TestCTo012PricedDraftPickupDateUpdateDenied(t *testing.T) {
	order := &todomain.TransportOrder{PricingModelVersion: ptr("SNAPSHOT_V1")}
	pickup := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	err := todomain.ValidateUpdateWithSnapshot(order, todomain.UpdateTransportOrderInput{RequestedPickupDate: &pickup})
	if err == nil {
		t.Fatal("expected requested_pickup_date update denial")
	}
}

func TestCTo014PublicCreateCannotCreateNullPricingModel(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "to-014")
	result, err := env.pricedOrders.CreatePricedOrder(ctx, in, sampleSnapshot(tenantID, buyerID, carrierID, originID, destID), testRequestHash("to014"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var pricingModel *string
	if err := env.pool.QueryRow(ctx, `SELECT pricing_model_version FROM transport.transport_orders WHERE id = $1`, result.Order.ID).Scan(&pricingModel); err != nil {
		t.Fatalf("query: %v", err)
	}
	if pricingModel == nil || *pricingModel != todomain.PricingModelVersionSnapshotV1 {
		t.Fatalf("priced create must persist SNAPSHOT_V1, got %v", pricingModel)
	}
}

func TestCTo016ConcurrentSameKeySameRequest(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantID, buyerID, carrierID, originID, destID, cargoID := seedTenantCompanies(t, env.pool)
	in := sampleCreateInput(tenantID, buyerID, carrierID, originID, destID, cargoID, "concurrent-key")
	hash, err := todomain.ComputeCreateRequestHash(in)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	snap := sampleSnapshot(tenantID, buyerID, carrierID, originID, destID)

	const workers = 3
	var wg sync.WaitGroup
	results := make([]*todomain.PricedTransportOrderResult, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = env.pricedOrders.WithCreateIdempotencyLock(ctx, tenantID, buyerID, in.IdempotencyKey, func(lockCtx context.Context) (*todomain.PricedTransportOrderResult, error) {
				return env.pricedOrders.CreatePricedOrder(lockCtx, in, snap, hash)
			})
		}(i)
	}
	wg.Wait()
	for i, callErr := range errs {
		if callErr != nil {
			t.Fatalf("worker %d: %v", i, callErr)
		}
	}
	for i := 1; i < workers; i++ {
		if results[0].Order.ID != results[i].Order.ID || results[0].RateSnapshot.ID != results[i].RateSnapshot.ID {
			t.Fatalf("concurrent idempotent create returned divergent results")
		}
	}
}

func TestCTo017CrossTenantSnapshotFKCorruptionDBDeny(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	tenantA := seedSecondTenantCompanies(t, env.pool)
	tenantB := seedSecondTenantCompanies(t, env.pool)
	orderA := insertBareTransportOrder(t, env.pool, tenantA.tenantID, tenantA.buyerID, tenantA.originID, tenantA.destID, tenantA.cargoID, "TO-A")
	snapB := sampleSnapshot(tenantB.tenantID, tenantB.buyerID, tenantB.carrierID, tenantB.originID, tenantB.destID)
	var snapshotBID uuid.UUID
	err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, manual_spot_audit_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'MANUAL_SPOT',$5,$6,$7,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',1500,CURRENT_DATE,now(),'test','v2.0C',$8)
		RETURNING id`,
		tenantB.tenantID, orderA, tenantB.buyerID, tenantB.carrierID, snapB.ManualSpotAuditID, tenantB.originID, tenantB.destID, strings.Repeat("z", 64),
	).Scan(&snapshotBID)
	if err == nil {
		t.Fatal("expected cross-tenant snapshot parent FK denial")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("expected FK violation, got %v", err)
	}

	orderB := insertBareTransportOrder(t, env.pool, tenantB.tenantID, tenantB.buyerID, tenantB.originID, tenantB.destID, tenantB.cargoID, "TO-B")
	var snapshotInB uuid.UUID
	err = env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, manual_spot_audit_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'MANUAL_SPOT',$5,$6,$7,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',1500,CURRENT_DATE,now(),'test','v2.0C',$8)
		RETURNING id`,
		tenantB.tenantID, orderB, tenantB.buyerID, tenantB.carrierID, snapB.ManualSpotAuditID, tenantB.originID, tenantB.destID, strings.Repeat("y", 64),
	).Scan(&snapshotInB)
	if err != nil {
		t.Fatalf("seed tenant B snapshot: %v", err)
	}

	_, err = env.pool.Exec(ctx, `
		INSERT INTO transport.transport_order_create_idempotency (
			tenant_id, actor_company_id, idempotency_key, request_hash, transport_order_id, rate_snapshot_id
		) VALUES ($1,$2,'corrupt-key',$3,$4,$5)`,
		tenantA.tenantID, tenantA.buyerID, strings.Repeat("k", 64), orderA, snapshotInB)
	if err == nil {
		t.Fatal("expected cross-tenant idempotency snapshot FK denial")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("expected idempotency FK violation, got %v", err)
	}
}
