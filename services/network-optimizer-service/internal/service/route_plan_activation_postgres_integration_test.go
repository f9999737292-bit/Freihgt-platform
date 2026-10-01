//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
)

func TestNLO04C_PostgresActivation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	pool := startConsolidationPostgres(t, ctx)
	for _, name := range []string{
		"000001_create_schemas.up.sql",
		"000003_create_transport_tables.up.sql",
		"000075_bno_capacity_marketplace_foundation_v0_1a.up.sql",
		"000076_bno_predictive_capacity_v0_1b.up.sql",
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
		"000080_bno_match_score_topn_v0_1c2.up.sql",
		"000081_nlo_pairwise_consolidation_v0_3b.up.sql",
		"000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql",
		"000083_nlo_current_trip_fill_v0_3d.up.sql",
		"000084_nlo_bounded_n_member_search_v0_3e.up.sql",
		"000085_nlo_route_plan_bounded_planner_v0_4b.up.sql",
		"000086_nlo_route_plan_accept_activate_v0_4c.up.sql",
		// Current evaluation reads the service-duration policy. 000094 is compatible with the 0.4C schema.
		"000094_nlo_service_duration_policy_v0_4d.up.sql",
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	store := repository.NewPostgres(pool)
	svc := New(store, nil)
	svc.UseRouting(&countingRoute{})
	dir := &dirMap{snaps: map[uuid.UUID]domain.LocationSnapshot{}}
	svc.UseDirectory(dir)
	now := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })
	carrier := uuid.New()
	shipper := uuid.New()
	origin, dest := uuid.New(), uuid.New()
	lat, lon := 55.1, 37.2
	win := domain.TimeWindow{Start: &now, End: timePtr(now.Add(24 * time.Hour))}
	load := domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: shipper, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup:       domain.Place{LocationID: &origin, Latitude: &lat, Longitude: &lon},
		Delivery:     domain.Place{LocationID: &dest, Latitude: f64(56), Longitude: f64(38)},
		PickupWindow: win, DeliveryWindow: win, VisibilityScope: domain.VisMarketplace, Status: domain.LoadPublished,
		Version: 1, WeightKg: f64(100), VolumeM3: f64(1), CrossShipperConsolidationAllowed: true,
		CreatedAt: now, UpdatedAt: now,
	}
	location := uuid.New()
	cap := domain.Capacity{
		ID: uuid.New(), OwnerTenantID: carrier, LocationID: &location, Latitude: &lat, Longitude: &lon,
		AvailableFrom: now, AvailableUntil: now.Add(time.Hour), Source: domain.SourceManual,
		VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 1,
		PayloadRemainingKg: f64(10000), VolumeRemainingM3: f64(40), CreatedAt: now, UpdatedAt: now,
	}
	if err := store.Within(ctx, func(tx repository.Tx) error {
		if err := tx.InsertLoad(ctx, load); err != nil {
			return err
		}
		return tx.InsertCapacity(ctx, cap)
	}); err != nil {
		t.Fatal(err)
	}
	actor := Actor{TenantID: carrier, UserID: uuid.New()}
	shipment := uuid.New()
	vehicle := uuid.New()
	cargo := uuid.New()
	tripDest := uuid.New()
	dir.snaps[tripDest] = domain.LocationSnapshot{ID: tripDest, Latitude: f64(57), Longitude: f64(39)}
	posLat, posLon := 54.0, 36.0
	recorded := now
	src := &fillSources{
		execution: currenttrip.ShipmentExecution{
			ShipmentID: shipment, TenantID: carrier, ShipmentVersion: 2, ShipmentStatus: "IN_TRANSIT",
			VehicleID: &vehicle, OriginLocationID: uuid.New(), DestinationLocationID: tripDest,
		},
		onboard: currenttrip.OnboardCargo{ShipmentID: shipment, ShipmentVersion: 2, Items: []currenttrip.OnboardEvidenceItem{{
			CargoID: cargo, State: currenttrip.EvidenceConfirmedOnboard, StateVersion: 1, OccurredAt: now, Source: "DRIVER_OPERATION",
		}}},
		profiles: map[uuid.UUID]currenttrip.CargoProfile{cargo: {ID: cargo, Version: 3, WeightKg: f64(400)}},
		vehicle:  currenttrip.VehicleCapability{ID: vehicle, TenantID: carrier, Version: 1, CapacityWeight: f64(20000)},
		position: currenttrip.TrackingPosition{Freshness: "FRESH", Latitude: &posLat, Longitude: &posLon, RecordedAt: &recorded},
		eta:      currenttrip.ETA{FreshnessStatus: "FRESH"},
	}
	svc.ConfigureCurrentTrip(currenttrip.NewProvider(src, src, src, src, src, src))
	planA := evaluateTrip(t, svc, actor, shipment, load.ID, "pg-eval-a")
	makePlanActivatable(t, ctx, pool, planA, now)
	if _, err := svc.AcceptRoutePlan(ctx, actor, "accept-a", planA, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	first, err := svc.ActivateRoutePlan(ctx, actor, "activate-a", planA, []byte(`{"version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	activationA := activationID(t, first.Body)
	if countActivations(t, ctx, pool, carrier) != 1 || planStatus(t, ctx, pool, planA) != routeplan.StatusAccepted || activationStatus(t, ctx, pool, activationA) != routeplan.ActivationPending || !pendingLinkageNull(t, ctx, pool, activationA) {
		t.Fatalf("first activation count=%d plan=%s activation=%s", countActivations(t, ctx, pool, carrier), planStatus(t, ctx, pool, planA), activationStatus(t, ctx, pool, activationA))
	}
	if supported(t, ctx, pool, planA) {
		t.Fatal("execution_supported became true")
	}
	replay, err := svc.ActivateRoutePlan(ctx, actor, "activate-a", planA, []byte(`{"version":1}`))
	if err != nil || activationID(t, replay.Body) != activationA || countActivations(t, ctx, pool, carrier) != 1 || activationStatus(t, ctx, pool, activationA) != routeplan.ActivationPending || !pendingLinkageNull(t, ctx, pool, activationA) {
		t.Fatalf("replay %v count=%d status=%s", err, countActivations(t, ctx, pool, carrier), activationStatus(t, ctx, pool, activationA))
	}
	again, err := svc.ActivateRoutePlan(ctx, actor, "activate-b", planA, []byte(`{"version":1}`))
	if err != nil || activationID(t, again.Body) != activationA || countActivations(t, ctx, pool, carrier) != 1 || activationStatus(t, ctx, pool, activationA) != routeplan.ActivationPending {
		t.Fatalf("new key %v id=%s count=%d status=%s", err, activationID(t, again.Body), countActivations(t, ctx, pool, carrier), activationStatus(t, ctx, pool, activationA))
	}
	if _, err := svc.ActivateRoutePlan(ctx, Actor{TenantID: uuid.New(), UserID: uuid.New()}, "foreign", planA, []byte(`{"version":1}`)); err == nil {
		t.Fatal("foreign tenant activated")
	} else {
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeNotFound {
			t.Fatal(err)
		}
	}
	if src.execution.ShipmentStatus != "IN_TRANSIT" {
		t.Fatalf("shipment mutated %s", src.execution.ShipmentStatus)
	}

	executionID := uuid.New()
	revisionID := uuid.New()
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.route_plan_activations
		SET status='EXECUTION_LINKED', execution_id=$2, execution_revision_id=$3, effective_shipment_id=$4
		WHERE id=$1`, uuid.MustParse(activationA), executionID, revisionID, shipment); err != nil {
		t.Fatal(err)
	}
	planB := evaluateTrip(t, svc, actor, shipment, load.ID, "pg-eval-b")
	makePlanActivatable(t, ctx, pool, planB, now)
	if _, err := pool.Exec(ctx, `UPDATE network_optimizer.route_plans SET supersedes_plan_id=$2 WHERE id=$1`, planB, planA); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AcceptRoutePlan(ctx, actor, "accept-b", planB, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	successor, err := svc.ActivateRoutePlan(ctx, actor, "activate-successor", planB, []byte(`{"version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	successorID := activationID(t, successor.Body)
	if activationStatus(t, ctx, pool, successorID) != routeplan.ActivationPending || !pendingLinkageNull(t, ctx, pool, successorID) || planStatus(t, ctx, pool, planA) != routeplan.StatusAccepted || effectiveCount(t, ctx, pool, carrier, shipment) != 1 {
		t.Fatalf("successor status=%s planA=%s effective=%d", activationStatus(t, ctx, pool, successorID), planStatus(t, ctx, pool, planA), effectiveCount(t, ctx, pool, carrier, shipment))
	}

	planC := evaluateTrip(t, svc, actor, shipment, load.ID, "pg-eval-c")
	makePlanActivatable(t, ctx, pool, planC, now)
	if _, err := svc.AcceptRoutePlan(ctx, actor, "accept-c", planC, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("fail before commit")
	err = store.Within(ctx, func(tx repository.Tx) error {
		if err := tx.InsertRoutePlanActivation(ctx, repository.RoutePlanActivationRow{
			ID: uuid.New(), TenantID: carrier, RoutePlanID: planC, PlanVersion: 1, Version: 1, IdempotencyKey: "rolled-back",
			ExecutionShipmentID: &shipment, Status: routeplan.ActivationPending, CreatedAt: now,
		}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if planStatus(t, ctx, pool, planA) != routeplan.StatusAccepted || effectiveCount(t, ctx, pool, carrier, shipment) != 1 || activationExists(t, ctx, pool, planC) {
		t.Fatalf("rollback leaked planA=%s effective=%d c=%v", planStatus(t, ctx, pool, planA), effectiveCount(t, ctx, pool, carrier, shipment), activationExists(t, ctx, pool, planC))
	}

	planD := evaluateTrip(t, svc, actor, shipment, load.ID, "pg-eval-d")
	makePlanActivatable(t, ctx, pool, planD, now)
	if _, err := svc.AcceptRoutePlan(ctx, actor, "accept-d", planD, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	const racers = 6
	var wg sync.WaitGroup
	ids := make([]string, racers)
	errs := make([]error, racers)
	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func(i int) {
			defer wg.Done()
			result, err := svc.ActivateRoutePlan(ctx, actor, "race-"+string(rune('a'+i)), planD, []byte(`{"version":1}`))
			errs[i] = err
			if err == nil {
				ids[i] = activationID(t, result.Body)
			}
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("racer %d: %v", i, err)
		}
		if ids[i] == "" || ids[i] != ids[0] {
			t.Fatalf("ids %v", ids)
		}
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.route_plan_activations WHERE route_plan_id=$1`, planD).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("concurrent rows %d %v", rows, err)
	}
	if effectiveCount(t, ctx, pool, carrier, shipment) != 1 || planStatus(t, ctx, pool, planA) != routeplan.StatusAccepted || activationStatus(t, ctx, pool, ids[0]) != routeplan.ActivationPending {
		t.Fatalf("effective after race %d planA=%s", effectiveCount(t, ctx, pool, carrier, shipment), planStatus(t, ctx, pool, planA))
	}

	left := evaluateTrip(t, svc, actor, shipment, load.ID, "pg-eval-left")
	right := evaluateTrip(t, svc, actor, shipment, load.ID, "pg-eval-right")
	makePlanActivatable(t, ctx, pool, left, now)
	makePlanActivatable(t, ctx, pool, right, now)
	if _, err := pool.Exec(ctx, `UPDATE network_optimizer.route_plans SET supersedes_plan_id=$2 WHERE id=$1 OR id=$3`, left, planD, right); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AcceptRoutePlan(ctx, actor, "accept-left", left, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AcceptRoutePlan(ctx, actor, "accept-right", right, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	var pair sync.WaitGroup
	pairErr := make([]error, 2)
	pair.Add(2)
	for i, planID := range []uuid.UUID{left, right} {
		go func(i int, planID uuid.UUID) {
			defer pair.Done()
			_, pairErr[i] = svc.ActivateRoutePlan(ctx, actor, "pair-"+string(rune('a'+i)), planID, []byte(`{"version":1}`))
		}(i, planID)
	}
	pair.Wait()
	wins := 0
	for _, err := range pairErr {
		if err == nil {
			wins++
		}
	}
	if wins != 2 || effectiveCount(t, ctx, pool, carrier, shipment) != 1 || planStatus(t, ctx, pool, planA) != routeplan.StatusAccepted {
		t.Fatalf("successor race wins=%d errs=%v effective=%d planA=%s", wins, pairErr, effectiveCount(t, ctx, pool, carrier, shipment), planStatus(t, ctx, pool, planA))
	}
}

func evaluateTrip(t *testing.T, svc *Service, actor Actor, shipment, load uuid.UUID, key string) uuid.UUID {
	t.Helper()
	body := `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + shipment.String() + `","candidate_load_ids":["` + load.String() + `"]}`
	var cmd RoutePlanCommand
	if err := json.Unmarshal([]byte(body), &cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Raw = []byte(body)
	created, err := svc.EvaluateRoutePlan(context.Background(), actor, key, cmd)
	if err != nil || created.Status != http.StatusCreated {
		t.Fatalf("evaluate %s: %v %+v", key, err, created)
	}
	var doc map[string]any
	if err := json.Unmarshal(created.Body, &doc); err != nil {
		t.Fatal(err)
	}
	return uuid.MustParse(doc["id"].(string))
}

func makePlanActivatable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, now time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.route_plans
		SET result_status='FEASIBLE_PLAN_FOUND', reason_codes='{}'
		WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.route_plan_stops
		SET service_duration_seconds=60
		WHERE route_plan_id=$1 AND stop_role='CARGO'`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.route_plan_legs
		SET expires_at=$2
		WHERE route_plan_id=$1`, id, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE network_optimizer.route_capacity_snapshots
		SET compatibility_status='COMPATIBLE'
		WHERE route_plan_id=$1 AND compatibility_status='INDETERMINATE'`, id); err != nil {
		t.Fatal(err)
	}
}

func activationID(t *testing.T, body []byte) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	activation, _ := doc["activation"].(map[string]any)
	id, _ := activation["id"].(string)
	if id == "" {
		t.Fatalf("activation missing %s", body)
	}
	return id
}

func countActivations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.route_plan_activations WHERE tenant_id=$1`, tenant).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func effectiveCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant, shipment uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM network_optimizer.route_plan_activations
		WHERE tenant_id=$1 AND effective_shipment_id=$2 AND status='EXECUTION_LINKED'`, tenant, shipment).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func planStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM network_optimizer.route_plans WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func pendingLinkageNull(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) bool {
	t.Helper()
	var executionID, revisionID, effective *uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT execution_id, execution_revision_id, effective_shipment_id
		FROM network_optimizer.route_plan_activations WHERE id=$1`, uuid.MustParse(id)).Scan(&executionID, &revisionID, &effective); err != nil {
		t.Fatal(err)
	}
	return executionID == nil && revisionID == nil && effective == nil
}

func activationStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM network_optimizer.route_plan_activations WHERE id=$1`, uuid.MustParse(id)).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func activationExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, planID uuid.UUID) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.route_plan_activations WHERE route_plan_id=$1`, planID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func supported(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) bool {
	t.Helper()
	var value bool
	if err := pool.QueryRow(ctx, `SELECT execution_supported FROM network_optimizer.route_plans WHERE id=$1`, id).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
