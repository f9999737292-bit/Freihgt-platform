//go:build integration

package driverlocation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/tracking-service/internal/config"
	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/provider"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

func TestExecutionStopTracking(t *testing.T) {
	env := setupTestEnv(t)
	if err := applyMigrationNumbers(env.pool, 27, 91); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	execRepo := repository.NewExecutionTrackingRepository(env.pool)
	trackingRepo := repository.NewTrackingRepository(env.pool)
	etaRepo := repository.NewETARepository(env.pool)
	cfg := config.Config{
		FreshnessPolicy:    config.FreshnessConfig{FreshThreshold: 5 * time.Minute, StaleThreshold: 30 * time.Minute},
		ETAFreshnessPolicy: config.FreshnessConfig{FreshThreshold: 15 * time.Minute, StaleThreshold: 60 * time.Minute},
		StopApproach:       config.StopApproachConfig{Enabled: true, RadiusMeters: 300},
	}
	telemetry := env.metrics
	etaEvaluator := service.NewETAStateEvaluator(etaRepo, cfg)
	etaIngest := service.NewETAIngestService(trackingRepo, etaRepo, provider.NewETARegistry(provider.GenericETAAdapter{}), cfg, etaEvaluator, log, telemetry)
	etaIngest.SetExecutionTracking(execRepo)
	etaQuery := service.NewETAQueryService(etaRepo, etaEvaluator)
	etaQuery.SetExecutionTracking(execRepo)
	approach := service.NewStopApproachService(trackingRepo, execRepo, true, 300)
	reg := provider.NewRegistry(provider.GenericAdapter{})
	evaluator := service.NewStateEvaluator(trackingRepo, cfg)
	ingest := service.NewIngestService(trackingRepo, reg, cfg, evaluator, log, telemetry)
	ingest.SetApproach(approach)

	t.Run("CONSUMER", func(t *testing.T) {
		operating := uuid.New()
		execution := uuid.New()
		revision := uuid.New()
		anchor := uuid.New()
		cargo := uuid.New()
		next := uuid.New()
		driver := uuid.New()
		lat, lon := 55.81, 37.71
		client := &fakeStopContext{views: map[uuid.UUID]service.StopContext{
			anchor: {ExecutionID: execution, RevisionID: revision, ExecutionStopID: anchor, Ordinal: 0, Status: "PLANNED", PointKind: "POSITION_ANCHOR", StopRole: "START", DriverID: &driver, LiveETAStopID: &cargo, LiveETAOrdinal: intPtr(1), LiveETALatitude: &lat, LiveETALongitude: &lon},
			cargo:  {ExecutionID: execution, RevisionID: revision, ExecutionStopID: cargo, Ordinal: 1, Status: "PLANNED", PointKind: "CANONICAL_LOCATION", StopRole: "CARGO", DriverID: &driver, TargetLatitude: &lat, TargetLongitude: &lon, LiveETAStopID: &cargo, LiveETAOrdinal: intPtr(1), LiveETALatitude: &lat, LiveETALongitude: &lon},
			next:   {ExecutionID: execution, RevisionID: revision, ExecutionStopID: next, Ordinal: 2, Status: "PLANNED", PointKind: "CANONICAL_LOCATION", StopRole: "CARGO", DriverID: &driver, TargetLatitude: &lat, TargetLongitude: &lon, LiveETAStopID: &next, LiveETAOrdinal: intPtr(2), LiveETALatitude: &lat, LiveETALongitude: &lon},
		}}
		consumer := service.NewExecutionConsumer(execRepo, client)
		raw := eventJSON(t, operating, execution, revision, anchor, 1, domain.EventRouteStopCurrent)
		if err := consumer.Apply(ctx, raw); err != nil {
			t.Fatal(err)
		}
		if client.tenants[0] != operating {
			t.Fatal("context was not read with the operating tenant")
		}
		state := mustState(t, ctx, execRepo, execution)
		if state.CurrentStopID == nil || *state.CurrentStopID != anchor || state.LiveETAStopID == nil || *state.LiveETAStopID != cargo {
			t.Fatalf("target %+v", state)
		}
		if err := consumer.Apply(ctx, raw); err != nil {
			t.Fatal(err)
		}
		if countQuery(t, ctx, env.pool, `SELECT count(*) FROM tracking.execution_event_inbox WHERE execution_id=$1`, execution) != 1 {
			t.Fatal("duplicate event was not idempotent")
		}
		if err := consumer.Apply(ctx, eventJSON(t, operating, execution, revision, next, 3, domain.EventRouteStopCurrent)); err != nil {
			t.Fatal(err)
		}
		state = mustState(t, ctx, execRepo, execution)
		if state.LiveETAStopID == nil || *state.LiveETAStopID != next || state.LastExecutionEventSequence != 3 {
			t.Fatalf("next target %+v", state)
		}
		if err := consumer.Apply(ctx, eventJSON(t, operating, execution, revision, cargo, 2, domain.EventRouteStopCurrent)); err != nil {
			t.Fatal(err)
		}
		state = mustState(t, ctx, execRepo, execution)
		if state.LiveETAStopID == nil || *state.LiveETAStopID != next {
			t.Fatal("old event regressed the target")
		}
		if err := consumer.Apply(ctx, eventJSON(t, uuid.New(), execution, revision, next, 4, domain.EventRouteStopCurrent)); err != nil {
			t.Fatal(err)
		}
		state = mustState(t, ctx, execRepo, execution)
		if state.OperatingTenantID != operating || state.LastExecutionEventSequence != 3 {
			t.Fatal("foreign tenant event was applied")
		}
		if err := consumer.Apply(ctx, eventJSON(t, operating, execution, revision, next, 5, domain.EventRouteStopCompleted)); err != nil {
			t.Fatal(err)
		}
		state = mustState(t, ctx, execRepo, execution)
		if state.CurrentStopID != nil || state.LiveETAStopID != nil {
			t.Fatalf("completed event did not clear the target %+v", state)
		}
		missingExec := uuid.New()
		client.missing = map[uuid.UUID]bool{missingExec: true}
		if err := consumer.Apply(ctx, eventJSON(t, operating, missingExec, revision, uuid.New(), 1, domain.EventRouteStopCurrent)); err != nil {
			t.Fatal(err)
		}
		if countQuery(t, ctx, env.pool, `SELECT count(*) FROM tracking.execution_tracking_state WHERE execution_id=$1`, missingExec) != 0 {
			t.Fatal("superseded context created a target")
		}
	})

	t.Run("ETA", func(t *testing.T) {
		operating := uuid.New()
		execution := uuid.New()
		stop := uuid.New()
		other := uuid.New()
		driver := uuid.New()
		vehicle := uuid.New()
		planned := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
		insertState(t, ctx, env.pool, operating, execution, uuid.New(), stop, &driver, &vehicle, &planned, 55.75, 37.62)
		device := "eta-device"
		insertBinding(t, ctx, env.pool, uuid.New(), uuid.New(), &driver, &vehicle, "generic", device)
		observed := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
		estimated := observed.Add(30 * time.Minute)
		payload := etaPayload(device, stop, estimated, observed, "evt-eta-1")
		first, err := etaIngest.IngestProviderETA(ctx, "generic", payload)
		if err != nil || first.Accepted != 1 {
			t.Fatalf("ingest %+v err %v", first, err)
		}
		second, err := etaIngest.IngestProviderETA(ctx, "generic", payload)
		if err != nil || second.Deduplicated != 1 || second.Accepted != 0 {
			t.Fatalf("dedup %+v err %v", second, err)
		}
		view, err := etaQuery.GetExecutionStopETA(ctx, operating, stop)
		if err != nil {
			t.Fatal(err)
		}
		if view.PlannedArrival == nil || !view.PlannedArrival.Equal(planned) || view.EstimatedArrivalAt == nil || view.EstimatedArrivalAt.Equal(planned) {
			t.Fatalf("planned and live eta were not separate %+v", view)
		}
		if view.FreshnessStatus != domain.ETAFreshnessFresh || view.QualityStatus != domain.ETAQualityGood || view.Status != domain.ETAStatusAvailable {
			t.Fatalf("freshness/quality %+v", view)
		}
		if _, err := etaQuery.GetExecutionStopETA(ctx, uuid.New(), stop); err == nil {
			t.Fatal("foreign tenant eta was returned")
		}
		stale, err := etaIngest.IngestProviderETA(ctx, "generic", etaPayload(device, other, estimated, observed, "evt-stale"))
		if err != nil || stale.Rejected != 1 || stale.Accepted != 0 {
			t.Fatalf("stale stop %+v", stale)
		}
		var plannedDB time.Time
		if err := env.pool.QueryRow(ctx, `SELECT planned_arrival FROM tracking.execution_stop_eta_state WHERE execution_stop_id=$1`, stop).Scan(&plannedDB); err != nil {
			t.Fatal(err)
		}
		if !plannedDB.Equal(planned) {
			t.Fatal("live eta overwrote planned arrival")
		}
		otherExec := uuid.New()
		insertState(t, ctx, env.pool, operating, otherExec, uuid.New(), uuid.New(), &driver, &vehicle, &planned, 55.75, 37.62)
		ambiguous, err := etaIngest.IngestProviderETA(ctx, "generic", etaPayload(device, stop, estimated.Add(time.Minute), observed, "evt-ambiguous"))
		if err != nil || ambiguous.Rejected != 1 || ambiguous.Accepted != 0 {
			t.Fatalf("ambiguous device association %+v", ambiguous)
		}

		shipTenant := uuid.New()
		shipment := uuid.New()
		pickupDevice := "pickup-device"
		insertBinding(t, ctx, env.pool, shipTenant, shipment, nil, nil, "generic", pickupDevice)
		pickupObserved := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
		pickup, err := etaIngest.IngestProviderETA(ctx, "generic", provider.ProviderPayload([]byte(fmt.Sprintf(`{"observations":[{"providerDeviceId":%q,"targetType":"pickup","estimatedArrivalAt":%q,"sourceObservedAt":%q,"sourceType":"provider_eta","providerEventId":"pick-1"}]}`, pickupDevice, pickupObserved.Add(time.Hour).Format(time.RFC3339), pickupObserved.Format(time.RFC3339)))))
		if err != nil || pickup.Accepted != 1 {
			t.Fatalf("pickup %+v err %v", pickup, err)
		}
		var execStop *uuid.UUID
		var shipmentID uuid.UUID
		if err := env.pool.QueryRow(ctx, `SELECT shipment_id, execution_stop_id FROM tracking.eta_observation WHERE provider_event_id='pick-1'`).Scan(&shipmentID, &execStop); err != nil {
			t.Fatal(err)
		}
		if shipmentID != shipment || execStop != nil {
			t.Fatalf("pickup shape shipment %s stop %v", shipmentID, execStop)
		}
		delivery, err := etaIngest.IngestProviderETA(ctx, "generic", provider.ProviderPayload([]byte(fmt.Sprintf(`{"observations":[{"providerDeviceId":%q,"targetType":"delivery","estimatedArrivalAt":%q,"sourceObservedAt":%q,"sourceType":"provider_eta","providerEventId":"del-1"}]}`, pickupDevice, pickupObserved.Add(2*time.Hour).Format(time.RFC3339), pickupObserved.Format(time.RFC3339)))))
		if err != nil || delivery.Accepted != 1 {
			t.Fatalf("delivery %+v err %v", delivery, err)
		}
	})

	t.Run("APPROACH", func(t *testing.T) {
		operating := uuid.New()
		outsideExec := uuid.New()
		outsideDriver := uuid.New()
		insertState(t, ctx, env.pool, operating, outsideExec, uuid.New(), uuid.New(), &outsideDriver, nil, nil, 55.7558, 37.6173)
		outsideDevice := "outside-device"
		insertBinding(t, ctx, env.pool, uuid.New(), uuid.New(), &outsideDriver, nil, "generic", outsideDevice)
		far := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
		outsideResult, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(outsideDevice, 55.90, 37.90, far))
		if err != nil || outsideResult.Accepted != 1 {
			t.Fatalf("outside ingest %+v err %v", outsideResult, err)
		}
		if approachCount(t, ctx, env.pool, outsideExec) != 0 {
			t.Fatal("outside radius emitted an approach")
		}

		execution := uuid.New()
		stop := uuid.New()
		driver := uuid.New()
		insertState(t, ctx, env.pool, operating, execution, uuid.New(), stop, &driver, nil, nil, 55.7558, 37.6173)
		device := "approach-device"
		insertBinding(t, ctx, env.pool, uuid.New(), uuid.New(), &driver, nil, "generic", device)
		near := far.Add(time.Second)
		insideResult, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(device, 55.7560, 37.6173, near))
		if err != nil || insideResult.Accepted != 1 {
			t.Fatalf("inside ingest %+v err %v", insideResult, err)
		}
		if approachCount(t, ctx, env.pool, execution) != 1 {
			t.Fatal("inside radius did not emit one approach")
		}
		payload := approachPayload(t, ctx, env.pool, execution)
		if _, ok := payload["latitude"]; ok {
			t.Fatal("approach payload included latitude")
		}
		if payload["executionStopId"] != stop.String() || payload["operatingTenantId"] != operating.String() {
			t.Fatalf("payload %+v", payload)
		}
		if _, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(device, 55.7560, 37.6173, near)); err != nil {
			t.Fatal(err)
		}
		if _, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(device, 55.7562, 37.6174, near.Add(time.Second))); err != nil {
			t.Fatal(err)
		}
		if approachCount(t, ctx, env.pool, execution) != 1 {
			t.Fatal("approach was emitted more than once")
		}
		next := uuid.New()
		if _, err := env.pool.Exec(ctx, `UPDATE tracking.execution_tracking_state SET live_eta_stop_id=$2, approach_emitted_stop_id=NULL, approach_emitted_at=NULL WHERE execution_id=$1`, execution, next); err != nil {
			t.Fatal(err)
		}
		if _, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(device, 55.7561, 37.6173, near.Add(2*time.Second))); err != nil {
			t.Fatal(err)
		}
		if approachCount(t, ctx, env.pool, execution) != 2 {
			t.Fatal("new current stop did not emit a new approach")
		}
		if countQuery(t, ctx, env.pool, `SELECT count(*) FROM transport.shipment_event_outbox WHERE event_type='tracking.stop.approaching'`) != 0 {
			t.Fatal("approach event was written to the shipment outbox")
		}

		unknownDriver := uuid.New()
		unknownExec := uuid.New()
		unknownStop := uuid.New()
		insertState(t, ctx, env.pool, operating, unknownExec, uuid.New(), unknownStop, &unknownDriver, nil, nil, 0, 0)
		if _, err := env.pool.Exec(ctx, `UPDATE tracking.execution_tracking_state SET target_latitude=NULL, target_longitude=NULL WHERE execution_id=$1`, unknownExec); err != nil {
			t.Fatal(err)
		}
		unknownDevice := "unknown-device"
		insertBinding(t, ctx, env.pool, uuid.New(), uuid.New(), &unknownDriver, nil, "generic", unknownDevice)
		if _, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(unknownDevice, 55.7560, 37.6173, near)); err != nil {
			t.Fatal(err)
		}
		if approachCount(t, ctx, env.pool, unknownExec) != 0 {
			t.Fatal("unknown coordinates emitted an approach")
		}

		staleDriver := uuid.New()
		staleDevice := "stale-device"
		staleExec := uuid.New()
		insertState(t, ctx, env.pool, operating, staleExec, uuid.New(), uuid.New(), &staleDriver, nil, nil, 55.7558, 37.6173)
		insertBinding(t, ctx, env.pool, uuid.New(), uuid.New(), &staleDriver, nil, "generic", staleDevice)
		if _, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(staleDevice, 55.7560, 37.6173, time.Now().UTC().Add(-20*time.Minute))); err != nil {
			t.Fatal(err)
		}
		if _, err := ingest.IngestProviderLocations(ctx, "generic", locationPayload(staleDevice, 55.7560, 37.6173, time.Now().UTC().Add(-2*time.Hour))); err != nil {
			t.Fatal(err)
		}
		if approachCount(t, ctx, env.pool, staleExec) != 0 {
			t.Fatal("stale or lost gps emitted an approach")
		}

		restore := repository.SetTrackingOutboxInsertForTest(func(context.Context, pgx.Tx, repository.TrackingOutboxEvent) error {
			return fmt.Errorf("injected outbox failure")
		})
		defer restore()
		failExec := uuid.New()
		failStop := uuid.New()
		failDriver := uuid.New()
		insertState(t, ctx, env.pool, operating, failExec, uuid.New(), failStop, &failDriver, nil, nil, 55.7558, 37.6173)
		before := countQuery(t, ctx, env.pool, `SELECT count(*) FROM tracking.location_event WHERE driver_id=$1`, failDriver)
		event := domain.LocationEvent{
			ID: uuid.New(), TenantID: uuid.New(), ShipmentID: uuid.New(), DriverID: &failDriver,
			ProviderCode: "generic", ProviderDeviceID: "fail-device", DedupKey: uuid.NewString(),
			Latitude: 55.7560, Longitude: 37.6173, RecordedAt: time.Now().UTC().Add(-time.Minute), ReceivedAt: time.Now().UTC(),
			SourceType: "gps", QualityStatus: domain.QualityGood,
		}
		if _, err := approach.AcceptLocation(ctx, event, domain.FreshnessFresh, domain.QualityGood); err == nil {
			t.Fatal("outbox failure was ignored")
		}
		if countQuery(t, ctx, env.pool, `SELECT count(*) FROM tracking.location_event WHERE driver_id=$1`, failDriver) != before {
			t.Fatal("outbox failure committed the location")
		}
		if approachCount(t, ctx, env.pool, failExec) != 0 {
			t.Fatal("outbox failure committed the approach")
		}
	})
}

type fakeStopContext struct {
	views   map[uuid.UUID]service.StopContext
	missing map[uuid.UUID]bool
	tenants []uuid.UUID
}

func (f *fakeStopContext) Get(_ context.Context, operatingTenantID, executionID, stopID uuid.UUID) (service.StopContext, error) {
	f.tenants = append(f.tenants, operatingTenantID)
	if f.missing[executionID] {
		return service.StopContext{}, service.ContextNotFoundError{}
	}
	view, ok := f.views[stopID]
	if !ok {
		return service.StopContext{}, service.ContextNotFoundError{}
	}
	return view, nil
}

func eventJSON(t *testing.T, operating, execution, revision, stop uuid.UUID, sequence int64, eventType string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"event_id": uuid.NewString(), "event_type": eventType, "operating_tenant_id": operating.String(),
		"execution_id": execution.String(), "revision_id": revision.String(), "revision_version": 1,
		"event_sequence": sequence, "stop_id": stop.String(), "occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func insertState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, operating, execution, revision, stop uuid.UUID, driver, vehicle *uuid.UUID, planned *time.Time, lat, lon float64) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO tracking.execution_tracking_state (
  operating_tenant_id, execution_id, revision_id, current_stop_id, current_stop_ordinal,
  planned_arrival, target_latitude, target_longitude, live_eta_stop_id, live_eta_ordinal,
  driver_id, vehicle_id, last_execution_event_sequence
) VALUES ($1,$2,$3,$4,1,$5,$6,$7,$4,1,$8,$9,1)
`, operating, execution, revision, stop, planned, lat, lon, driver, vehicle)
	if err != nil {
		t.Fatal(err)
	}
}

func insertBinding(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant, shipment uuid.UUID, driver, vehicle *uuid.UUID, providerCode, device string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO tracking.shipment_tracking_binding (
  id, tenant_id, shipment_id, vehicle_id, driver_id, provider_code, provider_device_id, status
) VALUES ($1,$2,$3,$4,$5,$6,$7,'active')
`, uuid.New(), tenant, shipment, vehicle, driver, providerCode, device)
	if err != nil {
		t.Fatal(err)
	}
}

func etaPayload(device string, stop uuid.UUID, estimated, observed time.Time, eventID string) provider.ProviderPayload {
	return provider.ProviderPayload([]byte(fmt.Sprintf(`{"observations":[{"providerDeviceId":%q,"targetType":"execution_stop","executionStopId":%q,"estimatedArrivalAt":%q,"sourceObservedAt":%q,"sourceType":"provider_eta","providerEventId":%q}]}`, device, stop, estimated.Format(time.RFC3339), observed.Format(time.RFC3339), eventID)))
}

func locationPayload(device string, lat, lon float64, recorded time.Time) provider.ProviderPayload {
	return provider.ProviderPayload([]byte(fmt.Sprintf(`{"events":[{"providerDeviceId":%q,"latitude":%f,"longitude":%f,"recordedAt":%q,"sourceType":"vehicle_telematics","accuracyMeters":10}]}`, device, lat, lon, recorded.Format(time.RFC3339))))
}

func mustState(t *testing.T, ctx context.Context, repo *repository.ExecutionTrackingRepository, execution uuid.UUID) *repository.ExecutionTrackingState {
	t.Helper()
	state, err := repo.GetState(ctx, execution)
	if err != nil || state == nil {
		t.Fatalf("state err %v", err)
	}
	return state
}

func countQuery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func approachCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, execution uuid.UUID) int {
	t.Helper()
	return countQuery(t, ctx, pool, `SELECT count(*) FROM tracking.event_outbox WHERE aggregate_id=$1 AND event_type='tracking.stop.approaching'`, execution)
}

func approachPayload(t *testing.T, ctx context.Context, pool *pgxpool.Pool, execution uuid.UUID) map[string]any {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM tracking.event_outbox WHERE aggregate_id=$1 AND event_type='tracking.stop.approaching' ORDER BY created_at LIMIT 1`, execution).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func intPtr(v int) *int { return &v }

func applyMigrationNumbers(pool *pgxpool.Pool, numbers ...int) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, number := range numbers {
		matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%06d_*.up.sql", number)))
		if err != nil || len(matches) != 1 {
			return fmt.Errorf("migration %06d matches %v err %v", number, matches, err)
		}
		content, err := os.ReadFile(matches[0])
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(matches[0]), err)
		}
	}
	return nil
}
