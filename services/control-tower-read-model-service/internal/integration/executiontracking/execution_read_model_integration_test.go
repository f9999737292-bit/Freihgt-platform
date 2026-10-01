//go:build integration

package executiontracking

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/http/handlers"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

func TestExecutionReadModel(t *testing.T) {
	ctx := context.Background()
	pool := startExecutionPostgres(t)
	repo := repository.NewExecutionProjectionRepository(pool)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	meta := domain.KafkaRecordMeta{Topic: "shipment.status.v1", Partition: 1, Offset: 1}

	if !tableExists(t, pool, "control_tower.shipment_status_projection") {
		t.Fatal("shipment status projection missing")
	}
	down, err := os.ReadFile(filepath.Join(migrationsDir(t), "000092_tms_control_tower_execution_projection_v0_1e.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	if tableExists(t, pool, "control_tower.execution_projection") {
		t.Fatal("down left execution projection")
	}
	if !tableExists(t, pool, "control_tower.shipment_status_projection") {
		t.Fatal("down removed shipment status projection")
	}
	up, err := os.ReadFile(filepath.Join(migrationsDir(t), "000092_tms_control_tower_execution_projection_v0_1e.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}

	operating := uuid.New()
	executionID := uuid.New()
	revisionID := uuid.New()
	stopA := uuid.New()
	stopB := uuid.New()
	actionA := uuid.New()
	actionB := uuid.New()
	shipA := uuid.New()
	shipB := uuid.New()
	cargoA := uuid.New()
	cargoB := uuid.New()
	carrier := uuid.New()

	created := planCreated(t, operating, executionID, revisionID, carrier, stopA, stopB, actionA, actionB, shipA, shipB, cargoA, cargoB, now)
	restore := repository.SetExecutionApplyFaultForTest(func() error { return fmt.Errorf("injected") })
	if err := repo.ApplyRecord(ctx, created, meta, now); err == nil {
		t.Fatal("fault was not injected")
	}
	restore()
	if countTable(t, pool, "control_tower.execution_projection") != 0 || countTable(t, pool, "control_tower.execution_event_inbox") != 0 {
		t.Fatal("fault did not roll back")
	}
	meta.Offset = 2
	if err := repo.ApplyRecord(ctx, created, meta, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyRecord(ctx, created, meta, now); err != nil {
		t.Fatal(err)
	}
	if countTable(t, pool, "control_tower.execution_projection") != 1 || countTable(t, pool, "control_tower.execution_stop_projection") != 2 || countTable(t, pool, "control_tower.execution_action_projection") != 2 {
		t.Fatal("plan created projection counts")
	}
	if columnExists(t, pool, "execution_action_projection", "shipment_tenant_id") || columnExists(t, pool, "execution_stop_projection", "latitude") {
		t.Fatal("private column stored")
	}

	current := stopEvent(t, "shipment.route_stop.current", operating, executionID, revisionID, stopA, 2, now)
	arrivedGap := stopEvent(t, "shipment.route_stop.arrived", operating, executionID, revisionID, stopA, 4, now.Add(time.Minute))
	service := stopEvent(t, "shipment.route_stop.service_started", operating, executionID, revisionID, stopA, 3, now.Add(2*time.Minute))
	meta.Offset = 3
	if err := repo.ApplyRecord(ctx, current, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset = 4
	if err := repo.ApplyRecord(ctx, arrivedGap, meta, now); err != nil {
		t.Fatal(err)
	}
	view := mustGet(t, repo, executionID)
	if !view.GapDetected || view.LastEventSequence != 2 || view.Stops[0].Status != "PLANNED" {
		t.Fatalf("gap not held: %+v %+v", view.GapDetected, view.Stops[0].Status)
	}
	meta.Offset = 5
	if err := repo.ApplyRecord(ctx, service, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.GapDetected || view.LastEventSequence != 4 || view.Stops[0].Status != "ARRIVED" || view.CurrentStopID == nil || *view.CurrentStopID != stopA {
		t.Fatalf("gap recovery regressed %+v status %s", view.LastEventSequence, view.Stops[0].Status)
	}
	firstArrived := *view.Stops[0].ArrivedAt
	meta.Offset = 6
	replayArrived := stopEvent(t, "shipment.route_stop.arrived", operating, executionID, revisionID, stopA, 4, now.Add(3*time.Hour))
	if err := repo.ApplyRecord(ctx, replayArrived, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if !view.Stops[0].ArrivedAt.Equal(firstArrived) {
		t.Fatal("arrived timestamp rewritten")
	}
	meta.Offset = 7
	old := stopEvent(t, "shipment.route_stop.current", operating, executionID, revisionID, stopB, 2, now)
	if err := repo.ApplyRecord(ctx, old, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.CurrentStopID == nil || *view.CurrentStopID != stopA || view.LastEventSequence != 4 {
		t.Fatal("old event regressed projection")
	}
	meta.Offset = 8
	completed := stopEvent(t, "shipment.route_stop.completed", operating, executionID, revisionID, stopA, 5, now.Add(4*time.Minute))
	if err := repo.ApplyRecord(ctx, completed, meta, now); err != nil {
		t.Fatal(err)
	}
	meta.Offset = 9
	override := stopEvent(t, "shipment.route_stop.sequence_overridden", operating, executionID, revisionID, stopA, 6, now.Add(5*time.Minute))
	if err := repo.ApplyRecord(ctx, override, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.Stops[0].Status != "COMPLETED" || view.CurrentStopID != nil {
		t.Fatalf("final stop status=%s current=%v", view.Stops[0].Status, view.CurrentStopID)
	}
	if countTable(t, pool, "control_tower.shipment_status_projection") != 0 {
		t.Fatal("execution events mutated shipment status projection")
	}

	approach := []byte(`{"eventId":"` + uuid.NewString() + `","eventType":"tracking.stop.approaching","operatingTenantId":"` + operating.String() + `","executionId":"` + executionID.String() + `","revisionId":"` + revisionID.String() + `","executionStopId":"` + stopB.String() + `","occurredAt":"` + now.Format(time.RFC3339Nano) + `","distanceMeters":120}`)
	meta.Topic = "tracking.events.v1"
	meta.Offset = 1
	if err := repo.ApplyRecord(ctx, approach, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.Stops[1].Status != "PLANNED" || view.Stops[1].ApproachingAt == nil || view.Stops[1].ApproachDistanceMeters == nil {
		t.Fatal("approach was not advisory")
	}
	if err := repo.ApplyRecord(ctx, approach, meta, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	again := mustGet(t, repo, executionID)
	if !again.Stops[1].ApproachingAt.Equal(*view.Stops[1].ApproachingAt) {
		t.Fatal("approach replay rewrote timestamp")
	}
	foreign := []byte(`{"eventId":"` + uuid.NewString() + `","eventType":"tracking.stop.approaching","operatingTenantId":"` + uuid.NewString() + `","executionId":"` + executionID.String() + `","revisionId":"` + revisionID.String() + `","executionStopId":"` + stopB.String() + `","occurredAt":"` + now.Format(time.RFC3339Nano) + `"}`)
	meta.Offset = 2
	if err := repo.ApplyRecord(ctx, foreign, meta, now); err != nil {
		t.Fatal(err)
	}
	unknownExec := []byte(`{"eventId":"` + uuid.NewString() + `","eventType":"tracking.stop.approaching","operatingTenantId":"` + operating.String() + `","executionId":"` + uuid.NewString() + `","revisionId":"` + revisionID.String() + `","executionStopId":"` + uuid.NewString() + `","occurredAt":"` + now.Format(time.RFC3339Nano) + `"}`)
	meta.Offset = 3
	if err := repo.ApplyRecord(ctx, unknownExec, meta, now); err != nil {
		t.Fatal(err)
	}
	if countTable(t, pool, "control_tower.execution_projection") != 1 {
		t.Fatal("approach created a route root")
	}

	superseded := stopEvent(t, "shipment.execution_plan.superseded", operating, executionID, revisionID, uuid.Nil, 7, now.Add(6*time.Minute))
	meta.Topic = "shipment.status.v1"
	meta.Offset = 10
	if err := repo.ApplyRecord(ctx, superseded, meta, now); err != nil {
		t.Fatal(err)
	}
	view = mustGet(t, repo, executionID)
	if view.Stops[0].Status != "COMPLETED" || view.Stops[1].Status != "SUPERSEDED" {
		t.Fatalf("superseded history %s %s", view.Stops[0].Status, view.Stops[1].Status)
	}

	private := append([]byte(nil), created...)
	private = []byte(strings.Replace(string(created), `"stops"`, `"price":1,"stops"`, 1))
	meta.Offset = 11
	if err := repo.ApplyRecord(ctx, private, meta, now); err != nil {
		t.Fatal(err)
	}
	if countTable(t, pool, "control_tower.execution_projection") != 1 {
		t.Fatal("private payload created a projection")
	}

	if err := repo.LinkDriverContext(ctx, uuid.New(), uuid.New(), shipA, "driver.delay.reported", &stopA, nil, now, "TRAFFIC", "high"); err != nil {
		t.Fatal(err)
	}
	if err := repo.LinkDriverContext(ctx, uuid.New(), uuid.New(), shipB, "driver.problem.reported", &stopB, &actionB, now, "CARGO_ISSUE", "critical"); err != nil {
		t.Fatal(err)
	}
	otherShip := uuid.New()
	before := countTable(t, pool, "control_tower.execution_progress_event")
	if err := repo.LinkDriverContext(ctx, uuid.New(), uuid.New(), otherShip, "driver.delay.reported", &stopA, nil, now, "TRAFFIC", "high"); err != nil {
		t.Fatal(err)
	}
	if countTable(t, pool, "control_tower.execution_progress_event") != before {
		t.Fatal("cross-shipment stop was attached")
	}

	scoped := callerOwner{allowed: map[uuid.UUID]map[uuid.UUID]bool{}}
	shipperATenant := uuid.New()
	shipperBTenant := uuid.New()
	scoped.allowed[shipperATenant] = map[uuid.UUID]bool{shipA: true}
	scoped.allowed[shipperBTenant] = map[uuid.UUID]bool{shipB: true}
	scopedHandler := handlers.NewExecutionHandler(repo, scoped)
	scopedRouter := chi.NewRouter()
	scopedRouter.Get("/internal/v1/control-tower/executions", scopedHandler.List)
	scopedRouter.Get("/internal/v1/control-tower/executions/{executionId}", scopedHandler.Get)
	carrierBody := getJSON(t, scopedRouter, "/internal/v1/control-tower/executions/"+executionID.String(), operating)
	if strings.Contains(carrierBody, "shipment_tenant_id") || !strings.Contains(carrierBody, shipB.String()) {
		t.Fatalf("carrier view %s", carrierBody)
	}
	bodyA := getJSON(t, scopedRouter, "/internal/v1/control-tower/executions/"+executionID.String(), shipperATenant)
	bodyB := getJSON(t, scopedRouter, "/internal/v1/control-tower/executions/"+executionID.String(), shipperBTenant)
	if strings.Contains(bodyA, shipB.String()) || strings.Contains(bodyA, cargoB.String()) || !strings.Contains(bodyA, shipA.String()) {
		t.Fatalf("shipper A saw B: %s", bodyA)
	}
	if strings.Contains(bodyB, shipA.String()) || strings.Contains(bodyB, cargoA.String()) || !strings.Contains(bodyB, shipB.String()) {
		t.Fatalf("shipper B saw A: %s", bodyB)
	}
	list := getJSON(t, scopedRouter, "/internal/v1/control-tower/executions", operating)
	if !strings.Contains(list, executionID.String()) {
		t.Fatalf("operating list %s", list)
	}
	foreignList := getJSON(t, scopedRouter, "/internal/v1/control-tower/executions", uuid.New())
	if strings.Contains(foreignList, executionID.String()) {
		t.Fatal("foreign list leaked execution")
	}
	missing := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/executions/"+uuid.NewString(), nil)
	missing.Header.Set("X-Tenant-ID", operating.String())
	rec := httptest.NewRecorder()
	scopedRouter.ServeHTTP(rec, missing)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing status %d", rec.Code)
	}
	foreignGet := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/control-tower/executions/"+executionID.String(), nil)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	scopedRouter.ServeHTTP(foreignGet, req)
	if foreignGet.Code != http.StatusNotFound {
		t.Fatalf("foreign detail %d", foreignGet.Code)
	}
}

type callerOwner struct {
	allowed map[uuid.UUID]map[uuid.UUID]bool
}

func (c callerOwner) Owns(_ context.Context, tenantID, shipmentID uuid.UUID) (bool, error) {
	return c.allowed[tenantID][shipmentID], nil
}

func getJSON(t *testing.T, handler http.Handler, path string, tenant uuid.UUID) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Tenant-ID", tenant.String())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		return ""
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status %d body %s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func mustGet(t *testing.T, repo *repository.ExecutionProjectionRepository, id uuid.UUID) repository.ExecutionView {
	t.Helper()
	view, err := repo.Get(context.Background(), id)
	if err != nil || view == nil {
		t.Fatalf("get %v %v", err, view)
	}
	return *view
}

func planCreated(t *testing.T, operating, executionID, revisionID, carrier, stopA, stopB, actionA, actionB, shipA, shipB, cargoA, cargoB uuid.UUID, now time.Time) []byte {
	t.Helper()
	body := map[string]any{
		"event_id": uuid.NewString(), "event_type": "shipment.execution_plan.created",
		"operating_tenant_id": operating.String(), "execution_id": executionID.String(),
		"revision_id": revisionID.String(), "revision_version": 1, "event_sequence": 1,
		"occurred_at": now.Format(time.RFC3339Nano), "carrier_company_id": carrier.String(),
		"stops": []map[string]any{
			{"stop_id": stopA.String(), "ordinal": 0, "stop_role": "START", "point_kind": "POSITION_ANCHOR", "status": "PLANNED"},
			{"stop_id": stopB.String(), "ordinal": 1, "stop_role": "CARGO", "point_kind": "CANONICAL_LOCATION", "status": "PLANNED"},
		},
		"actions": []map[string]any{
			{"action_id": actionA.String(), "stop_id": stopA.String(), "action_type": "PICKUP", "shipment_id": shipA.String(), "cargo_id": cargoA.String(), "status": "PENDING"},
			{"action_id": actionB.String(), "stop_id": stopB.String(), "action_type": "DELIVERY", "shipment_id": shipB.String(), "cargo_id": cargoB.String(), "status": "PENDING"},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func stopEvent(t *testing.T, eventType string, operating, executionID, revisionID, stopID uuid.UUID, seq int, occurred time.Time) []byte {
	t.Helper()
	body := map[string]any{
		"event_id": uuid.NewString(), "event_type": eventType, "operating_tenant_id": operating.String(),
		"execution_id": executionID.String(), "revision_id": revisionID.String(), "event_sequence": seq,
		"occurred_at": occurred.Format(time.RFC3339Nano),
	}
	if stopID != uuid.Nil {
		body["stop_id"] = stopID.String()
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func startExecutionPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	t.Cleanup(cancel)
	port := freePort(t)
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(port).
		RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
		DataPath(filepath.Join(t.TempDir(), "pg-data")).
		Database("freight_ct_test").
		Username("freight").
		Password("freight").
		Version(embeddedpostgres.V16).
		Logger(io.Discard))
	if err := pg.Start(); err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/freight_ct_test?sslmode=disable", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrationsThrough(ctx, pool, 92); err != nil {
		t.Fatal(err)
	}
	return pool
}

func applyMigrationsThrough(ctx context.Context, pool *pgxpool.Pool, max int) error {
	dir, err := locateCTMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		num := 0
		if _, scanErr := fmt.Sscanf(base, "%d", &num); scanErr != nil {
			return scanErr
		}
		if num > max {
			continue
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("%s: %w", base, execErr)
		}
	}
	return nil
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := locateCTMigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func columnExists(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), `
SELECT EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='control_tower' AND table_name=$1 AND column_name=$2
)`, table, column).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func countTable(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func freePort(t *testing.T) uint32 {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return uint32(ln.Addr().(*net.TCPAddr).Port)
}
