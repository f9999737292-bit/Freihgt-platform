//go:build integration

package transportexecution

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	shipmenthttp "github.com/freight-platform/shipment-service/internal/http"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestOperationsAnalyticsSourceContract(t *testing.T) {
	env := startPostgres(t)
	if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000093_tms_delivery_disposition_v0_1.up.sql")); err != nil {
		t.Fatal(err)
	}
	equalAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	earlyAt := equalAt.Add(-2 * time.Hour)
	lateAt := equalAt.Add(3 * time.Hour)

	tenantA := seedTenant(t, env, "analytics-a")
	graphA := seedAnalyticsGraph(t, env, tenantA)
	current := insertAnalyticsShipment(t, env, tenantA, graphA, "CREATED", nil, nil, false)
	insertAnalyticsShipment(t, env, tenantA, graphA, "DELIVERED", &equalAt, &equalAt, true)
	insertAnalyticsShipment(t, env, tenantA, graphA, "DELIVERED", &equalAt, &equalAt, false)
	insertAnalyticsShipment(t, env, tenantA, graphA, "DELIVERED", &equalAt, &earlyAt, false)
	insertAnalyticsShipment(t, env, tenantA, graphA, "DELIVERED", &equalAt, &lateAt, false)
	insertAnalyticsShipment(t, env, tenantA, graphA, "CREATED", &equalAt, nil, false)
	insertAnalyticsShipment(t, env, tenantA, graphA, "CREATED", nil, &equalAt, false)
	execA := projectAnalyticsExecution(t, env, tenantA, graphA, current)
	insertAnalyticsCase(t, env, tenantA, execA, domain.DispositionReturnToOrigin)
	insertAnalyticsCase(t, env, tenantA, execA, domain.DispositionReturnToOrigin)
	insertAnalyticsCase(t, env, tenantA, execA, domain.DispositionRedirect)
	insertAnalyticsCase(t, env, tenantA, execA, domain.DispositionHold)
	insertAnalyticsCase(t, env, tenantA, execA, "")

	tenantB := seedTenant(t, env, "analytics-b")
	graphB := seedAnalyticsGraph(t, env, tenantB)
	currentB := insertAnalyticsShipment(t, env, tenantB, graphB, "DELIVERED", &equalAt, &equalAt, false)
	insertAnalyticsShipment(t, env, tenantB, graphB, "CREATED", nil, nil, false)
	execB := projectAnalyticsExecution(t, env, tenantB, graphB, currentB)
	insertAnalyticsCase(t, env, tenantB, execB, domain.DispositionReturnToOrigin)
	insertAnalyticsCase(t, env, tenantB, execB, domain.DispositionRedirect)

	router := shipmenthttp.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), env.pool, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "analytics-token", nil)
	before := readAnalyticsSource(t, router, tenantA)
	wantA := domain.OperationsAnalyticsSourceSnapshot{
		TenantID: tenantA, ShipmentTotal: 6, OnTimeDeliveryDenominator: 3, OnTimeDeliveryNumerator: 2, ReturnCaseCount: 2, RedirectCaseCount: 1,
	}
	if before != wantA {
		t.Fatalf("tenant A %+v", before)
	}
	if again := readAnalyticsSource(t, router, tenantA); again != wantA {
		t.Fatalf("tenant A changed %+v", again)
	}
	wantB := domain.OperationsAnalyticsSourceSnapshot{
		TenantID: tenantB, ShipmentTotal: 2, OnTimeDeliveryDenominator: 1, OnTimeDeliveryNumerator: 1, ReturnCaseCount: 1, RedirectCaseCount: 1,
	}
	if got := readAnalyticsSource(t, router, tenantB); got != wantB {
		t.Fatalf("tenant B %+v", got)
	}
	emptyID := uuid.New()
	if got := readAnalyticsSource(t, router, emptyID); got != (domain.OperationsAnalyticsSourceSnapshot{TenantID: emptyID}) {
		t.Fatalf("empty %+v", got)
	}
	assertAnalyticsIndexPlan(t, env, tenantA)
}

func readAnalyticsSource(t *testing.T, router http.Handler, tenantID uuid.UUID) domain.OperationsAnalyticsSourceSnapshot {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/analytics/operations-foundation?tenant_id="+uuid.NewString(), nil)
	req.Header.Set("X-Internal-Service-Token", "analytics-token")
	req.Header.Set("X-Internal-Service-Name", "analytics-service")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sourceObservedAt") || strings.Contains(rec.Body.String(), "OPS_") {
		t.Fatalf("body %s", rec.Body.String())
	}
	var snap domain.OperationsAnalyticsSourceSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	return snap
}

type analyticsGraph struct {
	shipper   uuid.UUID
	consignee uuid.UUID
	carrier   uuid.UUID
	origin    uuid.UUID
	dest      uuid.UUID
	cargo     uuid.UUID
}

func seedAnalyticsGraph(t *testing.T, env *execEnv, tenantID uuid.UUID) analyticsGraph {
	t.Helper()
	graph := analyticsGraph{
		shipper:   seedCompany(t, env, tenantID, "SHIPPER", "Analytics shipper"),
		consignee: seedCompany(t, env, tenantID, "CONSIGNEE", "Analytics consignee"),
		carrier:   seedCompany(t, env, tenantID, "CARRIER", "Analytics carrier"),
		origin:    uuid.New(),
		dest:      uuid.New(),
		cargo:     uuid.New(),
	}
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{{graph.origin, "Analytics origin"}, {graph.dest, "Analytics destination"}} {
		if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, row.id, tenantID, row.name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description) VALUES ($1,$2,'GENERAL','Analytics cargo')`, graph.cargo, tenantID); err != nil {
		t.Fatal(err)
	}
	return graph
}

func insertAnalyticsShipment(t *testing.T, env *execEnv, tenantID uuid.UUID, graph analyticsGraph, status string, planned, actual *time.Time, deleted bool) seededShipment {
	t.Helper()
	orderID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, transport_mode
		) VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')
	`, orderID, tenantID, "TO-"+orderID.String()[:8], graph.shipper, graph.consignee, graph.origin, graph.dest); err != nil {
		t.Fatal(err)
	}
	shipmentID := uuid.New()
	var deletedAt *time.Time
	if deleted {
		at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		deletedAt = &at
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode, status,
			planned_delivery_at, actual_delivery_at, deleted_at, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,$11,$12,$13,1)
	`, shipmentID, tenantID, "SHP-"+shipmentID.String()[:8], orderID, graph.shipper, graph.consignee, graph.origin, graph.dest, graph.cargo, status, planned, actual, deletedAt); err != nil {
		t.Fatal(err)
	}
	return seededShipment{
		tenantID: tenantID, shipperID: graph.shipper, consigneeID: graph.consignee,
		originID: graph.origin, cargoID: graph.cargo, shipmentID: shipmentID, subjectID: uuid.New(),
	}
}

type analyticsExecution struct {
	execution uuid.UUID
	revision  uuid.UUID
	stop      uuid.UUID
	action    uuid.UUID
	shipment  seededShipment
}

func projectAnalyticsExecution(t *testing.T, env *execEnv, tenantID uuid.UUID, graph analyticsGraph, shipment seededShipment) analyticsExecution {
	t.Helper()
	version := 1
	stopID := uuid.New()
	duration := 600
	end := uuid.New()
	cmd := domain.ProjectionCommand{
		ActivationID: uuid.New(), ActivationVersion: 2, ActivationStatus: domain.ActivationStatusPendingExecution,
		RoutePlanID: uuid.New(), RoutePlanVersion: 4, PlanningMode: domain.PlanningModeCurrentTrip,
		OperatingTenantID: tenantID, ContextShipmentID: &shipment.shipmentID, ContextShipmentTenantID: &shipment.tenantID,
		ContextShipmentVersion: &version, CarrierCompanyID: graph.carrier, EvaluationFingerprint: "analytics-source",
		ExecutionSubjects: []domain.ProjectionSubject{materialSubject(shipment, version)},
		Stops: []domain.ProjectionStop{
			{RoutePlanStopID: uuid.New(), Ordinal: 0, StopRole: domain.StopRoleStart, PointKind: domain.PointKindPositionAnchor, Latitude: 55.7, Longitude: 37.6},
			{RoutePlanStopID: stopID, Ordinal: 1, StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &graph.origin, Latitude: 55.8, Longitude: 37.7, ServiceDurationSeconds: &duration},
			{RoutePlanStopID: end, Ordinal: 2, StopRole: domain.StopRoleEnd, PointKind: domain.PointKindCanonicalLocation, LocationID: &graph.dest, Latitude: 56, Longitude: 38},
		},
		Actions: []domain.ProjectionAction{materialAction(stopID, shipment, version, 0)},
	}
	result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	var stop uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_stops WHERE execution_id=$1 AND ordinal=1`, result.ExecutionID).Scan(&stop); err != nil {
		t.Fatal(err)
	}
	var action uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1`, stop).Scan(&action); err != nil {
		t.Fatal(err)
	}
	return analyticsExecution{execution: result.ExecutionID, revision: result.RevisionID, stop: stop, action: action, shipment: shipment}
}

func insertAnalyticsCase(t *testing.T, env *execEnv, tenantID uuid.UUID, exec analyticsExecution, dispositionType string) {
	t.Helper()
	var kind *string
	if dispositionType != "" {
		kind = &dispositionType
	}
	now := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.delivery_disposition_cases (
			id, operating_tenant_id, execution_id, source_revision_id, source_execution_stop_id, source_action_id,
			shipment_id, shipment_tenant_id, cargo_id,
			attempted_quantity, accepted_quantity, rejected_quantity,
			pending_quantity, return_reserved_quantity, redirect_reserved_quantity, hold_reserved_quantity, resolved_quantity,
			uom, reason_code, disposition_type, status,
			created_by_actor_kind, created_at, updated_at, version
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,
			1,0,1,1,0,0,0,0,
			'PALLET','DAMAGE',$10,'OPEN',
			'DRIVER',$11,$11,1
		)
	`, uuid.New(), tenantID, exec.execution, exec.revision, exec.stop, exec.action, exec.shipment.shipmentID, exec.shipment.tenantID, exec.shipment.cargoID, kind, now); err != nil {
		t.Fatal(err)
	}
}

func assertAnalyticsIndexPlan(t *testing.T, env *execEnv, tenantID uuid.UUID) {
	t.Helper()
	tx, err := env.pool.Begin(env.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(env.ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(env.ctx, `EXPLAIN `+repository.OperationsAnalyticsSourceStatement, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	text := plan.String()
	if strings.Contains(text, "Seq Scan on shipments") || strings.Contains(text, "Seq Scan on delivery_disposition_cases") {
		t.Fatalf("aggregate cannot use a tenant index:\n%s", text)
	}
	if !strings.Contains(text, "Index") {
		t.Fatalf("plan has no index:\n%s", text)
	}
	t.Logf("analytics source plan:\n%s", text)
}
