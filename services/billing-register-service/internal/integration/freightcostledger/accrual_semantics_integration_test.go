//go:build integration

package freightcostledger

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

type accrualFixture struct {
	TenantID   uuid.UUID
	BuyerID    uuid.UUID
	CarrierID  uuid.UUID
	UserID     uuid.UUID
	OriginID   uuid.UUID
	DestID     uuid.UUID
	CargoID    uuid.UUID
	OrderID    uuid.UUID
	SnapshotID uuid.UUID
	ShipmentID uuid.UUID
}

func seedAccrualFixture(t *testing.T, pool *pgxpool.Pool) accrualFixture {
	t.Helper()
	ctx := context.Background()
	fix := accrualFixture{
		TenantID:   uuid.New(),
		BuyerID:    uuid.New(),
		CarrierID:  uuid.New(),
		UserID:     uuid.New(),
		OriginID:   uuid.New(),
		DestID:     uuid.New(),
		CargoID:    uuid.New(),
		ShipmentID: uuid.New(),
	}
	if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		fix.TenantID, "t-"+fix.TenantID.String()[:8], "Accrual Semantics Tenant"); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{fix.BuyerID, fix.TenantID, "SHIPPER", "Buyer"},
		{fix.CarrierID, fix.TenantID, "CARRIER", "Carrier"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')`, row.id, row.tenant, row.typ, row.name); err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Origin"},
		{fix.DestID, "Destination"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, company_id, location_type, name, country_code, city)
			VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow')`, loc.id, fix.TenantID, fix.BuyerID, loc.name); err != nil {
			t.Fatalf("location: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description, gross_weight)
		VALUES ($1,$2,'GENERAL','Cargo',1000)`, fix.CargoID, fix.TenantID); err != nil {
		t.Fatalf("cargo: %v", err)
	}
	return fix
}

func seedSnapshotOrder(t *testing.T, env *env, fix *accrualFixture, totalAmount string) {
	t.Helper()
	ctx := context.Background()
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode, equipment_type,
			status, pricing_model_version
		) VALUES ($1,$2,$3,$3,$4,$5,$6,'ROAD','TAUTLINER','CONVERTED_TO_SHIPMENT','SNAPSHOT_V1')
		RETURNING id`, fix.TenantID, "TO-"+uuid.NewString()[:8], fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID).Scan(&fix.OrderID); err != nil {
		t.Fatalf("order: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_order_rate_snapshots (
			tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
			pricing_source, rfx_event_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
			currency_code, component_breakdown_status, components, accessorial_rules,
			total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
		) VALUES ($1,$2,$3,$4,'RFQ_AWARD',$5,$6,$7,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]','[]',
			$8,CURRENT_DATE,now(),'integration-test','v2.0C','dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd')
		RETURNING id`, fix.TenantID, fix.OrderID, fix.BuyerID, fix.CarrierID, uuid.New(), fix.OriginID, fix.DestID, totalAmount).Scan(&fix.SnapshotID); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, origin_location_id, destination_location_id, cargo_id, transport_mode, status,
			actual_delivery_at
		) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8,$9,'ROAD','DELIVERED',now())`,
		fix.ShipmentID, fix.TenantID, "SHP-"+fix.ShipmentID.String()[:8], fix.OrderID, fix.BuyerID,
		fix.CarrierID, fix.OriginID, fix.DestID, fix.CargoID); err != nil {
		t.Fatalf("shipment: %v", err)
	}
	podID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, created_at, updated_at
		) VALUES ($1,$2,$3,'POD','SIGNED',$4,'SHIPMENT',$5,now(),now())`,
		podID, fix.TenantID, "POD-"+fix.ShipmentID.String()[:8], fix.CarrierID, fix.ShipmentID); err != nil {
		t.Fatalf("pod: %v", err)
	}
}

func createSnapshotSettlement(t *testing.T, env *env, fix accrualFixture) *domain.FreightSettlement {
	t.Helper()
	settlement, err := env.settlements.Create(context.Background(), domain.CreateFreightSettlementInput{
		TenantID:         fix.TenantID,
		ShipmentID:       fix.ShipmentID,
		ActorUserID:      fix.UserID,
		ActorCompanyID:   fix.CarrierID,
		ActorKind:        domain.SettlementActorCarrier,
		IdempotencyKey:   "acc-sem-" + uuid.NewString(),
		SettlementNumber: "FS-" + fix.ShipmentID.String()[:8],
	})
	if err != nil {
		t.Fatalf("create settlement: %v", err)
	}
	return settlement
}

func querySnapshotTotal(t *testing.T, env *env, snapshotID uuid.UUID) string {
	t.Helper()
	var raw string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT total_amount::text FROM transport.transport_order_rate_snapshots WHERE id = $1`, snapshotID).Scan(&raw); err != nil {
		t.Fatalf("snapshot total: %v", err)
	}
	return raw
}

func querySettlementBase(t *testing.T, env *env, settlementID uuid.UUID) string {
	t.Helper()
	var raw string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT base_freight_amount::text FROM billing.freight_settlements WHERE id = $1`, settlementID).Scan(&raw); err != nil {
		t.Fatalf("base freight: %v", err)
	}
	return raw
}

func querySettlementRateSnapshotID(t *testing.T, env *env, settlementID uuid.UUID) *uuid.UUID {
	t.Helper()
	var id *uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `
		SELECT rate_snapshot_id FROM billing.freight_settlements WHERE id = $1`, settlementID).Scan(&id); err != nil {
		t.Fatalf("rate_snapshot_id: %v", err)
	}
	return id
}

func queryLatestAccrualOutboxAmount(t *testing.T, env *env, tenantID, settlementID uuid.UUID) string {
	t.Helper()
	var payload []byte
	err := env.pool.QueryRow(context.Background(), `
		SELECT payload FROM billing.freight_cost_outbox
		WHERE tenant_id = $1 AND aggregate_id = $2 AND event_type = $3
		ORDER BY source_revision DESC, created_at DESC
		LIMIT 1`, tenantID, settlementID, domain.EventFreightSettlementAccrualSnapshot).Scan(&payload)
	if err != nil {
		t.Fatalf("accrual outbox: %v", err)
	}
	var decoded struct {
		Amount *string `json:"amount"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal outbox: %v", err)
	}
	if decoded.Amount == nil {
		t.Fatal("accrual outbox amount is null")
	}
	return *decoded.Amount
}

func queryInternalAccrual(t *testing.T, env *env, tenantID, orderID uuid.UUID) string {
	t.Helper()
	read, err := env.repo.GetInternalByTransportOrder(context.Background(), tenantID, orderID)
	if err != nil {
		t.Fatalf("internal read: %v", err)
	}
	return read.AccrualAmountExVAT
}

func proposeAndApproveAccessorial(t *testing.T, env *env, fix accrualFixture, settlementID uuid.UUID, amount float64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	proposed, err := env.settlements.ProposeAccessorial(ctx, settlementID, domain.ProposeAccessorialInput{
		SettlementActorInput: domain.SettlementActorInput{
			TenantID: fix.TenantID, ActorUserID: fix.UserID, ActorCompanyID: fix.CarrierID, ActorKind: domain.SettlementActorCarrier,
		},
		ChargeCode: "LUMPER",
		Amount:     amount,
	})
	if err != nil {
		t.Fatalf("propose accessorial: %v", err)
	}
	if _, err := env.settlements.ApproveAccessorial(ctx, settlementID, proposed.ID, domain.SettlementActorInput{
		TenantID: fix.TenantID, ActorUserID: fix.UserID, ActorCompanyID: fix.BuyerID, ActorKind: domain.SettlementActorBuyer,
	}); err != nil {
		t.Fatalf("approve accessorial: %v", err)
	}
	return proposed.ID
}

func TestFC_B_ACC_SEM_A_SnapshotPrincipalEqualsSettlementBase(t *testing.T) {
	env := setupEnv(t)
	fix := seedAccrualFixture(t, env.pool)
	seedSnapshotOrder(t, env, &fix, "1000.00")
	settlement := createSnapshotSettlement(t, env, fix)

	snapshotTotal := querySnapshotTotal(t, env, fix.SnapshotID)
	base := querySettlementBase(t, env, settlement.ID)
	if snapshotTotal != "1000.00" || base != "1000.00" {
		t.Fatalf("snapshot=%s base=%s", snapshotTotal, base)
	}
	linked := querySettlementRateSnapshotID(t, env, settlement.ID)
	if linked == nil || *linked != fix.SnapshotID {
		t.Fatalf("settlement rate_snapshot_id=%v want %s", linked, fix.SnapshotID)
	}
}

func TestFC_B_ACC_SEM_B_ApproveAccessorialExactAccrualOutbox(t *testing.T) {
	env := setupEnv(t)
	fix := seedAccrualFixture(t, env.pool)
	seedSnapshotOrder(t, env, &fix, "1000.00")
	settlement := createSnapshotSettlement(t, env, fix)
	proposeAndApproveAccessorial(t, env, fix, settlement.ID, 100)

	got := queryLatestAccrualOutboxAmount(t, env, fix.TenantID, settlement.ID)
	if got != "1100.00" {
		t.Fatalf("accrual outbox = %s want 1100.00", got)
	}
}

func TestFC_B_ACC_SEM_C_DisputeRemovesAccrualBackToPrincipal(t *testing.T) {
	env := setupEnv(t)
	fix := seedAccrualFixture(t, env.pool)
	seedSnapshotOrder(t, env, &fix, "1000.00")
	settlement := createSnapshotSettlement(t, env, fix)
	accessorialID := proposeAndApproveAccessorial(t, env, fix, settlement.ID, 100)
	if got := queryLatestAccrualOutboxAmount(t, env, fix.TenantID, settlement.ID); got != "1100.00" {
		t.Fatalf("pre-dispute accrual = %s", got)
	}

	ctx := context.Background()
	if _, err := env.settlements.RaiseDispute(ctx, settlement.ID, domain.RaiseDisputeInput{
		SettlementActorInput: domain.SettlementActorInput{
			TenantID: fix.TenantID, ActorUserID: fix.UserID, ActorCompanyID: fix.BuyerID, ActorKind: domain.SettlementActorBuyer,
		},
		AccessorialID: &accessorialID,
		Reason:        "test dispute",
	}); err != nil {
		t.Fatalf("raise dispute: %v", err)
	}
	got := queryLatestAccrualOutboxAmount(t, env, fix.TenantID, settlement.ID)
	if got != "1000.00" {
		t.Fatalf("post-dispute accrual = %s want 1000.00", got)
	}
}

func TestFC_B_ACC_SEM_D_InternalReadMatchesLiveOutboxAccrual(t *testing.T) {
	env := setupEnv(t)
	fix := seedAccrualFixture(t, env.pool)
	seedSnapshotOrder(t, env, &fix, "1000.00")
	settlement := createSnapshotSettlement(t, env, fix)
	proposeAndApproveAccessorial(t, env, fix, settlement.ID, 100)

	live := queryLatestAccrualOutboxAmount(t, env, fix.TenantID, settlement.ID)
	rebuild := queryInternalAccrual(t, env, fix.TenantID, fix.OrderID)
	if live != rebuild || live != "1100.00" {
		t.Fatalf("live=%s rebuild-read=%s", live, rebuild)
	}
}

func TestFC_B_ACC_SEM_E_MissingSnapshotFailClosed(t *testing.T) {
	env := setupEnv(t)
	fix := seedAccrualFixture(t, env.pool)
	ctx := context.Background()
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO transport.transport_orders (
			tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode, equipment_type,
			status, pricing_model_version
		) VALUES ($1,$2,$3,$3,$4,$5,$6,'ROAD','TAUTLINER','CONVERTED_TO_SHIPMENT','SNAPSHOT_V1')
		RETURNING id`, fix.TenantID, "TO-MISSING", fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID).Scan(&fix.OrderID); err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, origin_location_id, destination_location_id, cargo_id, transport_mode, status,
			actual_delivery_at
		) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8,$9,'ROAD','DELIVERED',now())`,
		fix.ShipmentID, fix.TenantID, "SHP-MISSING", fix.OrderID, fix.BuyerID,
		fix.CarrierID, fix.OriginID, fix.DestID, fix.CargoID); err != nil {
		t.Fatalf("shipment: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, created_at, updated_at
		) VALUES ($1,$2,$3,'POD','SIGNED',$4,'SHIPMENT',$5,now(),now())`,
		uuid.New(), fix.TenantID, "POD-MISSING", fix.CarrierID, fix.ShipmentID); err != nil {
		t.Fatalf("pod: %v", err)
	}
	_, err := env.repo.LoadShipmentContext(ctx, fix.TenantID, fix.ShipmentID)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "snapshot") {
		t.Fatalf("expected fail-closed missing snapshot, got %v", err)
	}
}

func TestFC_B_ACC_SEM_F_BaseFreightImmutableAfterCreate(t *testing.T) {
	env := setupEnv(t)
	fix := seedAccrualFixture(t, env.pool)
	seedSnapshotOrder(t, env, &fix, "1000.00")
	settlement := createSnapshotSettlement(t, env, fix)
	baseBefore := querySettlementBase(t, env, settlement.ID)
	proposeAndApproveAccessorial(t, env, fix, settlement.ID, 250)
	baseAfter := querySettlementBase(t, env, settlement.ID)
	if baseBefore != baseAfter || baseBefore != "1000.00" {
		t.Fatalf("base changed: before=%s after=%s", baseBefore, baseAfter)
	}
}
