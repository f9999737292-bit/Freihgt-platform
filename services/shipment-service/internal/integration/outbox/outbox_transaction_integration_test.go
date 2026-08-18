//go:build integration

package outbox

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestMigrationCatalogChecks(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	assertTableExists(t, ctx, env.pool, "transport", "shipment_status_history")
	assertTableExists(t, ctx, env.pool, "transport", "shipment_event_outbox")

	var fkName *string
	err := env.pool.QueryRow(ctx, `
		SELECT conname FROM pg_constraint
		WHERE conrelid = 'transport.shipment_event_outbox'::regclass
		  AND contype = 'f'
		  AND conname = 'fk_shipment_event_outbox_history'
	`).Scan(&fkName)
	if err != nil || fkName == nil {
		t.Fatalf("FK fk_shipment_event_outbox_history missing: %v", err)
	}

	assertConstraintExists(t, ctx, env.pool, "transport.shipment_event_outbox", "uq_shipment_event_outbox_source_event")
	assertConstraintExists(t, ctx, env.pool, "transport.shipment_event_outbox", "chk_shipment_event_outbox_status")
	assertConstraintExists(t, ctx, env.pool, "transport.shipment_event_outbox", "chk_shipment_event_outbox_attempts")
	assertIndexExists(t, ctx, env.pool, "idx_shipment_event_outbox_pending")
	assertIndexExists(t, ctx, env.pool, "idx_shipment_event_outbox_tenant_aggregate")

	var roleCount int64
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM core.roles WHERE tenant_id IS NULL AND code = 'FORWARDER_MANAGER'
	`).Scan(&roleCount); err != nil {
		t.Fatalf("count forwarder role: %v", err)
	}
	if roleCount != 1 {
		t.Fatalf("FORWARDER_MANAGER count=%d want 1", roleCount)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO core.roles (tenant_id, code, name, description, scope, is_system)
		SELECT NULL, 'FORWARDER_MANAGER', 'Forwarder Manager', 'dup', 'TENANT', true
		WHERE NOT EXISTS (SELECT 1 FROM core.roles WHERE tenant_id IS NULL AND code = 'FORWARDER_MANAGER')
	`); err != nil {
		t.Fatalf("idempotent role seed failed: %v", err)
	}
	var roleCountAfter int64
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM core.roles WHERE tenant_id IS NULL AND code = 'FORWARDER_MANAGER'
	`).Scan(&roleCountAfter); err != nil {
		t.Fatalf("recount forwarder role: %v", err)
	}
	if roleCountAfter != 1 {
		t.Fatalf("duplicate role created: count=%d", roleCountAfter)
	}
}

func TestAtomicCreateShipmentHistoryOutbox(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipment, err := env.repo.CreateShipment(ctx, repository.CreateShipmentParams{
		TenantID:              fix.TenantID,
		ShipmentNumber:        "SHP-CREATE-1",
		TransportOrderID:      fix.TransportOrderID,
		ShipperCompanyID:      fix.ShipperID,
		ConsigneeCompanyID:    fix.ConsigneeID,
		CarrierCompanyID:      fix.CarrierID,
		OriginLocationID:      fix.OriginID,
		DestinationLocationID: fix.DestinationID,
		TransportMode:         "ROAD",
	}, userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create shipment: %v", err)
	}

	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipments WHERE id = $1`, shipment.ID) != 1 {
		t.Fatal("shipment count != 1")
	}
	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id = $1`, shipment.ID) != 1 {
		t.Fatal("history count != 1")
	}
	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id = $1`, shipment.ID) != 1 {
		t.Fatal("outbox count != 1")
	}

	var fromStatus *string
	var sourceEventID uuid.UUID
	var aggregateVersion int
	var tenantID uuid.UUID
	var aggregateID uuid.UUID
	var eventType string
	var outboxStatus string
	err = env.pool.QueryRow(ctx, `
		SELECT h.from_status, h.id, o.aggregate_version, o.tenant_id, o.aggregate_id, o.event_type, o.status
		FROM transport.shipment_status_history h
		JOIN transport.shipment_event_outbox o ON o.source_event_id = h.id
		WHERE h.shipment_id = $1
	`, shipment.ID).Scan(&fromStatus, &sourceEventID, &aggregateVersion, &tenantID, &aggregateID, &eventType, &outboxStatus)
	if err != nil {
		t.Fatalf("join history/outbox: %v", err)
	}
	if fromStatus != nil {
		t.Fatal("initial history from_status must be NULL")
	}
	if aggregateID != shipment.ID || aggregateVersion != shipment.Version || tenantID != fix.TenantID {
		t.Fatal("aggregate/tenant mismatch")
	}
	if eventType != domain.OutboxEventTypeCreated || outboxStatus != string(domain.OutboxStatusPending) {
		t.Fatalf("eventType=%s status=%s", eventType, outboxStatus)
	}
}

func TestAtomicStatusTransitionHistoryOutbox(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipment, err := env.repo.CreateShipment(ctx, repository.CreateShipmentParams{
		TenantID:              fix.TenantID,
		ShipmentNumber:        "SHP-TRANS-1",
		TransportOrderID:      fix.TransportOrderID,
		ShipperCompanyID:      fix.ShipperID,
		ConsigneeCompanyID:    fix.ConsigneeID,
		CarrierCompanyID:      fix.CarrierID,
		OriginLocationID:      fix.OriginID,
		DestinationLocationID: fix.DestinationID,
		TransportMode:         "ROAD",
	}, userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := env.repo.Accept(ctx, shipment.ID, fix.TenantID, shipment.Status, shipment.Version, userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if updated.Status != domain.ShipmentStatusAcceptedByCarrier {
		t.Fatalf("status=%s", updated.Status)
	}

	historyCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id = $1`, shipment.ID)
	outboxCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id = $1`, shipment.ID)
	if historyCount != 2 || outboxCount != 2 {
		t.Fatalf("history=%d outbox=%d want 2/2", historyCount, outboxCount)
	}

	var eventType string
	var aggregateVersion int
	var sourceEventID uuid.UUID
	err = env.pool.QueryRow(ctx, `
		SELECT o.event_type, o.aggregate_version, o.source_event_id
		FROM transport.shipment_event_outbox o
		JOIN transport.shipment_status_history h ON h.id = o.source_event_id
		WHERE h.shipment_id = $1 AND h.shipment_version = $2
	`, shipment.ID, updated.Version).Scan(&eventType, &aggregateVersion, &sourceEventID)
	if err != nil {
		t.Fatalf("lookup transition outbox: %v", err)
	}
	if aggregateVersion != updated.Version {
		t.Fatalf("aggregate version=%d want %d", aggregateVersion, updated.Version)
	}
	if eventType != domain.OutboxEventTypeStatusChanged {
		t.Fatalf("eventType=%s", eventType)
	}
}

func TestRollbackWhenOutboxInsertFails(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	const insertShipment = `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id,
			origin_location_id, destination_location_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,1)
		RETURNING id, version, status
	`
	shipmentID := uuid.New()
	var version int
	var status string
	if err := tx.QueryRow(ctx, insertShipment,
		shipmentID, fix.TenantID, "SHP-ROLLBACK-OUTBOX", fix.TransportOrderID,
		fix.ShipperID, fix.ConsigneeID, fix.CarrierID, fix.OriginID, fix.DestinationID,
		domain.ShipmentStatusCarrierAssigned,
	).Scan(&shipmentID, &version, &status); err != nil {
		t.Fatalf("insert shipment: %v", err)
	}

	write := repository.IntegrationHistoryWrite{
		TenantID:        fix.TenantID,
		ShipmentID:      shipmentID,
		ShipmentVersion: version,
		FromStatus:      nil,
		ToStatus:        domain.ShipmentStatusCarrierAssigned,
		Transition:      userTransition(fix.UserID),
	}
	history, err := repository.InsertStatusHistoryIntegration(ctx, tx, write)
	if err != nil {
		t.Fatalf("insert history: %v", err)
	}
	outbox, err := domain.BuildOutboxEventFromStatusHistory(history, nil)
	if err != nil {
		t.Fatalf("build outbox: %v", err)
	}
	if err := repository.InsertOutboxRowIntegration(ctx, tx, outbox); err != nil {
		t.Fatalf("first outbox insert: %v", err)
	}
	duplicate := outbox
	duplicate.ID = uuid.New()
	if err := repository.InsertOutboxRowIntegration(ctx, tx, duplicate); err == nil {
		t.Fatal("expected duplicate source_event_id violation")
	} else if !strings.Contains(strings.ToLower(err.Error()), "duplicate") &&
		!strings.Contains(strings.ToLower(err.Error()), "unique") &&
		!strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Fatalf("expected unique violation, got: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipments WHERE id = $1`, shipmentID) != 0 {
		t.Fatal("shipment must rollback")
	}
	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id = $1`, shipmentID) != 0 {
		t.Fatal("history must rollback")
	}
	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id = $1`, shipmentID) != 0 {
		t.Fatal("outbox must rollback")
	}
}

func TestRollbackWhenHistoryInsertFails(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	shipmentID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id,
			origin_location_id, destination_location_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,1)
	`, shipmentID, fix.TenantID, "SHP-ROLLBACK-HIST", fix.TransportOrderID,
		fix.ShipperID, fix.ConsigneeID, fix.CarrierID, fix.OriginID, fix.DestinationID,
		domain.ShipmentStatusCarrierAssigned)
	if err != nil {
		t.Fatalf("insert shipment: %v", err)
	}

	invalidActor := "INVALID"
	_, err = tx.Exec(ctx, `
		INSERT INTO transport.shipment_status_history (
			tenant_id, shipment_id, shipment_version, from_status, to_status, reason_code, source,
			actor_type, actor_id, correlation_id, occurred_at
		) VALUES ($1,$2,1,NULL,$3,NULL,'SHIPMENT_SERVICE',$4,NULL,NULL,now())
	`, fix.TenantID, shipmentID, domain.ShipmentStatusCarrierAssigned, invalidActor)
	if err == nil {
		t.Fatal("expected actor_type check violation")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipments WHERE id = $1`, shipmentID) != 0 {
		t.Fatal("shipment must rollback")
	}
	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id = $1`, shipmentID) != 0 {
		t.Fatal("history must be absent")
	}
	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id = $1`, shipmentID) != 0 {
		t.Fatal("outbox must be absent")
	}
}

func TestOptimisticLockConflictDoesNotWriteHistoryOrOutbox(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipment, err := env.repo.CreateShipment(ctx, repository.CreateShipmentParams{
		TenantID:              fix.TenantID,
		ShipmentNumber:        "SHP-OPT-1",
		TransportOrderID:      fix.TransportOrderID,
		ShipperCompanyID:      fix.ShipperID,
		ConsigneeCompanyID:    fix.ConsigneeID,
		CarrierCompanyID:      fix.CarrierID,
		OriginLocationID:      fix.OriginID,
		DestinationLocationID: fix.DestinationID,
		TransportMode:         "ROAD",
	}, userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	staleVersion := shipment.Version
	if _, err := env.repo.Accept(ctx, shipment.ID, fix.TenantID, shipment.Status, shipment.Version, userTransition(fix.UserID)); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	if _, err := env.repo.Accept(ctx, shipment.ID, fix.TenantID, domain.ShipmentStatusCarrierAssigned, staleVersion, userTransition(fix.UserID)); err == nil {
		t.Fatal("expected conflict on stale version")
	}

	historyCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_status_history WHERE shipment_id = $1`, shipment.ID)
	outboxCount := countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE aggregate_id = $1`, shipment.ID)
	if historyCount != 2 || outboxCount != 2 {
		t.Fatalf("history=%d outbox=%d want 2/2 after single accept", historyCount, outboxCount)
	}
}

func TestUniqueSourceEventConstraint(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipmentID := uuid.New()
	_, err := env.pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id,
			origin_location_id, destination_location_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,1)
	`, shipmentID, fix.TenantID, "SHP-UNIQ-1", fix.TransportOrderID,
		fix.ShipperID, fix.ConsigneeID, fix.CarrierID, fix.OriginID, fix.DestinationID,
		domain.ShipmentStatusCarrierAssigned)
	if err != nil {
		t.Fatalf("insert shipment: %v", err)
	}

	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	write := repository.IntegrationHistoryWrite{
		TenantID: fix.TenantID, ShipmentID: shipmentID, ShipmentVersion: 1,
		FromStatus: nil, ToStatus: domain.ShipmentStatusCarrierAssigned,
		Transition: userTransition(fix.UserID),
	}
	history, err := repository.InsertStatusHistoryIntegration(ctx, tx, write)
	if err != nil {
		t.Fatalf("insert history: %v", err)
	}
	outbox1, err := domain.BuildOutboxEventFromStatusHistory(history, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := repository.InsertOutboxRowIntegration(ctx, tx, outbox1); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	dup := outbox1
	dup.ID = uuid.New()
	if err := repository.InsertOutboxRowIntegration(ctx, tx, dup); err == nil {
		t.Fatal("expected unique violation")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if countRows(ctx, env.pool, `SELECT COUNT(*) FROM transport.shipment_event_outbox WHERE source_event_id = $1`, history.ID) != 0 {
		t.Fatal("rolled back duplicate attempt must leave zero outbox rows")
	}
}

func TestPayloadAndHeadersSafety(t *testing.T) {
	env := setupTestEnv(t)
	fix := env.seedFixture(t)
	ctx := context.Background()

	shipment, err := env.repo.CreateShipment(ctx, repository.CreateShipmentParams{
		TenantID:              fix.TenantID,
		ShipmentNumber:        "SHP-PAYLOAD-1",
		TransportOrderID:      fix.TransportOrderID,
		ShipperCompanyID:      fix.ShipperID,
		ConsigneeCompanyID:    fix.ConsigneeID,
		CarrierCompanyID:      fix.CarrierID,
		OriginLocationID:      fix.OriginID,
		DestinationLocationID: fix.DestinationID,
		TransportMode:         "ROAD",
	}, userTransition(fix.UserID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var payload, headers []byte
	if err := env.pool.QueryRow(ctx, `
		SELECT payload, headers FROM transport.shipment_event_outbox WHERE aggregate_id = $1 LIMIT 1
	`, shipment.ID).Scan(&payload, &headers); err != nil {
		t.Fatalf("load outbox json: %v", err)
	}
	lowerPayload := strings.ToLower(string(payload))
	for _, key := range []string{"eventid", "eventtype", "schemaversion", "occurredat", "tenantid", "aggregate", "sourceeventid", "data"} {
		if !strings.Contains(lowerPayload, key) {
			t.Fatalf("payload missing %s", key)
		}
	}
	for _, forbidden := range []string{"authorization", "jwt", "token", "password", "email", "phone", "fullname", "requestbody", "actorid"} {
		if strings.Contains(lowerPayload, forbidden) {
			t.Fatalf("payload contains forbidden %s", forbidden)
		}
	}
	lowerHeaders := strings.ToLower(string(headers))
	for _, forbidden := range []string{"authorization", "x-user-id", "x-user-email", "x-user-roles", "jwt", "bearer"} {
		if strings.Contains(lowerHeaders, forbidden) {
			t.Fatalf("headers contain forbidden %s", forbidden)
		}
	}
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if _, ok := envelope["data"]; !ok {
		t.Fatal("payload must contain data")
	}
}

func assertTableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, table string) {
	t.Helper()
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)
	`, schema, table).Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("table %s.%s missing: %v", schema, table, err)
	}
}

func assertConstraintExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, name string) {
	t.Helper()
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint WHERE conname = $1 AND conrelid = $2::regclass
		)
	`, name, table).Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("constraint %s on %s missing: %v", name, table, err)
	}
}

func assertIndexExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)
	`, name).Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("index %s missing: %v", name, err)
	}
}
