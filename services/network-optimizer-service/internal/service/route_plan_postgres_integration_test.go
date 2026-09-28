//go:build integration

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func TestNLO04B_PostgresRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
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
	counter := &countingRoute{}
	svc.UseRouting(counter)
	dir := &dirMap{snaps: map[uuid.UUID]domain.LocationSnapshot{}}
	svc.UseDirectory(dir)
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
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
		Commercial: domain.Commercial{Mode: "SECRET_TEST_MODE", Amount: f64(987654321.12), Currency: "ZZZ"},
		CreatedAt:  now, UpdatedAt: now,
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
	body := `{"planning_mode":"DEPOT_START","capacity_id":"` + cap.ID.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
	var cmd RoutePlanCommand
	if err := json.Unmarshal([]byte(body), &cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Raw = []byte(body)
	created, err := svc.EvaluateRoutePlan(ctx, actor, "pg-depot", cmd)
	if err != nil || created.Status != http.StatusCreated {
		t.Fatalf("evaluate %v %d", err, created.Status)
	}
	var doc map[string]any
	if err := json.Unmarshal(created.Body, &doc); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetRoutePlan(ctx, actor, uuid.MustParse(doc["id"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	var round map[string]any
	if err := json.Unmarshal(got.Body, &round); err != nil {
		t.Fatal(err)
	}
	if round["id"] != doc["id"] || len(round["stops"].([]any)) != len(doc["stops"].([]any)) || len(round["legs"].([]any)) == 0 || len(round["capacity_snapshots"].([]any)) == 0 || len(round["dependencies"].([]any)) == 0 {
		t.Fatalf("roundtrip mismatch %+v", round)
	}

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
	tripBody := `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + shipment.String() + `","candidate_load_ids":["` + load.ID.String() + `"]}`
	var tripCmd RoutePlanCommand
	if err := json.Unmarshal([]byte(tripBody), &tripCmd); err != nil {
		t.Fatal(err)
	}
	tripCmd.Raw = []byte(tripBody)
	tripCreated, err := svc.EvaluateRoutePlan(ctx, actor, "pg-trip", tripCmd)
	if err != nil {
		t.Fatal(err)
	}
	var trip map[string]any
	if err := json.Unmarshal(tripCreated.Body, &trip); err != nil {
		t.Fatal(err)
	}
	stops := trip["stops"].([]any)
	start := stops[0].(map[string]any)
	if start["point_kind"] != "POSITION_ANCHOR" || start["location_id"] != nil || start["point_observed_at"] == nil {
		t.Fatalf("anchor %+v", start)
	}
	reloaded, err := svc.GetRoutePlan(ctx, actor, uuid.MustParse(trip["id"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	var again map[string]any
	if err := json.Unmarshal(reloaded.Body, &again); err != nil {
		t.Fatal(err)
	}
	againStart := again["stops"].([]any)[0].(map[string]any)
	if againStart["latitude"] != start["latitude"] || againStart["longitude"] != start["longitude"] || againStart["location_id"] != nil {
		t.Fatalf("anchor roundtrip %+v vs %+v", againStart, start)
	}
	left, err := time.Parse(time.RFC3339, start["point_observed_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	right, err := time.Parse(time.RFC3339, againStart["point_observed_at"].(string))
	if err != nil || !left.Equal(right) {
		t.Fatalf("observed_at %v vs %v err %v", start["point_observed_at"], againStart["point_observed_at"], err)
	}
	depotID := uuid.MustParse(doc["id"].(string))
	tripID := uuid.MustParse(trip["id"].(string))
	var used bool
	var requestFP, responseFP string
	if err := pool.QueryRow(ctx, `SELECT provider_default_used, request_fingerprint, response_fingerprint FROM network_optimizer.route_plan_legs WHERE route_plan_id=$1 ORDER BY ordinal LIMIT 1`, depotID).Scan(&used, &requestFP, &responseFP); err != nil || !used || requestFP == "" || responseFP == "" || requestFP == responseFP {
		t.Fatalf("provider default roundtrip %v %s %s %v", used, requestFP, responseFP, err)
	}
	var snapshot []byte
	if err := pool.QueryRow(ctx, `SELECT public_subject_snapshot FROM network_optimizer.route_stop_actions WHERE route_plan_id=$1 AND public_subject_snapshot IS NOT NULL LIMIT 1`, depotID).Scan(&snapshot); err != nil || len(snapshot) == 0 {
		t.Fatalf("snapshot roundtrip %v %s", err, snapshot)
	}
	if strings.Contains(string(snapshot), shipper.String()) || strings.Contains(string(snapshot), "987654321") || strings.Contains(string(snapshot), "SECRET_TEST_MODE") || strings.Contains(string(snapshot), "ZZZ") || strings.Contains(string(snapshot), "commercial") || strings.Contains(string(snapshot), "owner_tenant_id") {
		t.Fatalf("unsafe marketplace snapshot %s", snapshot)
	}
	if !strings.Contains(string(got.Body), dest.String()) {
		t.Fatal("authorized marketplace geography missing")
	}
	if strings.Contains(string(got.Body), shipper.String()) || strings.Contains(string(got.Body), "SECRET_TEST_MODE") || strings.Contains(string(got.Body), "987654321") {
		t.Fatalf("unsafe marketplace response %s", got.Body)
	}
	var compatFP string
	if err := pool.QueryRow(ctx, `SELECT compatibility_fingerprint FROM network_optimizer.route_capacity_snapshots WHERE route_plan_id=$1 AND compatibility_fingerprint IS NOT NULL LIMIT 1`, tripID).Scan(&compatFP); err != nil || compatFP == "" {
		t.Fatalf("compatibility roundtrip %v %s", err, compatFP)
	}
	var catalogFP, ruleFP, algorithmFP string
	if err := pool.QueryRow(ctx, `SELECT fingerprint FROM network_optimizer.route_plan_dependencies WHERE route_plan_id=$1 AND dependency_kind='CATALOG'`, depotID).Scan(&catalogFP); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT fingerprint FROM network_optimizer.route_plan_dependencies WHERE route_plan_id=$1 AND dependency_kind='RULE_SET'`, depotID).Scan(&ruleFP); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT fingerprint FROM network_optimizer.route_plan_dependencies WHERE route_plan_id=$1 AND dependency_kind='ALGORITHM_POLICY'`, depotID).Scan(&algorithmFP); err != nil {
		t.Fatal(err)
	}
	if catalogFP == "" || ruleFP == "" || algorithmFP != "BOUNDED_INCREMENTAL_HEURISTIC/v0.4b" || catalogFP == algorithmFP {
		t.Fatalf("dependency fingerprints catalog %s rule %s algorithm %s", catalogFP, ruleFP, algorithmFP)
	}
	tag, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.route_plan_legs (
			id, route_plan_id, ordinal, from_stop_id, to_stop_id, from_point_fingerprint, to_point_fingerprint,
			distance_m, duration_seconds, provider, request_fingerprint, response_fingerprint,
			vehicle_profile_hash, route_mode, traffic_mode, calculated_at, expires_at, provider_default_used
		)
		SELECT gen_random_uuid(), $1, 99, s.id, s.id, 'from', 'to', 1, 1, 'script', 'req', 'res', 'veh', 'FASTEST', 'CURRENT', now(), now(), true
		FROM network_optimizer.route_plan_stops s WHERE s.route_plan_id=$2 LIMIT 1`, depotID, tripID)
	if err == nil || tag.RowsAffected() != 0 {
		t.Fatalf("cross-plan child insert succeeded %v rows %d", err, tag.RowsAffected())
	}
	secretID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	secretLat := 12.345678
	anonymized := domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: shipper, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup:       domain.Place{LocationID: &secretID, Latitude: &secretLat, Longitude: f64(77.654321), CountryCode: "RU", Region: "Secret", City: "Hidden"},
		Delivery:     domain.Place{LocationID: &secretID, Latitude: &secretLat, Longitude: f64(77.654321), CountryCode: "RU", Region: "Secret", City: "Hidden"},
		PickupWindow: win, DeliveryWindow: win, VisibilityScope: domain.VisAnonymized, Status: domain.LoadPublished,
		Version: 1, WeightKg: f64(50), VolumeM3: f64(1), CrossShipperConsolidationAllowed: true,
		Commercial: domain.Commercial{Mode: "SECRET_TEST_MODE", Amount: f64(987654321.12), Currency: "ZZZ"},
		CreatedAt:  now, UpdatedAt: now,
	}
	if err := store.Within(ctx, func(tx repository.Tx) error {
		return tx.InsertLoad(ctx, anonymized)
	}); err != nil {
		t.Fatal(err)
	}
	anonBody := `{"planning_mode":"DEPOT_START","capacity_id":"` + cap.ID.String() + `","candidate_load_ids":["` + anonymized.ID.String() + `"]}`
	var anonCmd RoutePlanCommand
	if err := json.Unmarshal([]byte(anonBody), &anonCmd); err != nil {
		t.Fatal(err)
	}
	anonCmd.Raw = []byte(anonBody)
	anonCreated, err := svc.EvaluateRoutePlan(ctx, actor, "pg-anon", anonCmd)
	if err != nil {
		t.Fatal(err)
	}
	var anonDoc map[string]any
	if err := json.Unmarshal(anonCreated.Body, &anonDoc); err != nil {
		t.Fatal(err)
	}
	anonGot, err := svc.GetRoutePlan(ctx, actor, uuid.MustParse(anonDoc["id"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	var anonSnapshot []byte
	if err := pool.QueryRow(ctx, `SELECT public_subject_snapshot FROM network_optimizer.route_stop_actions WHERE route_plan_id=$1 AND subject_type='LOAD_OPPORTUNITY' LIMIT 1`, uuid.MustParse(anonDoc["id"].(string))).Scan(&anonSnapshot); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{string(anonCreated.Body), string(anonGot.Body), string(anonSnapshot)} {
		if strings.Contains(raw, secretID.String()) || strings.Contains(raw, "12.345678") || strings.Contains(raw, "77.654321") || strings.Contains(raw, shipper.String()) || strings.Contains(raw, "SECRET_TEST_MODE") || strings.Contains(raw, "987654321") {
			t.Fatalf("anonymized privacy leak %s", raw)
		}
	}
}

func timePtr(value time.Time) *time.Time { return &value }
