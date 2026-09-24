//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMigration000076UpDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	if _, err := pool.Exec(ctx, mustRead(t, "000001_create_schemas.up.sql")); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000003_create_transport_tables.up.sql")); err != nil {
		t.Fatalf("transport: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000075_bno_capacity_marketplace_foundation_v0_1a.up.sql")); err != nil {
		t.Fatalf("000075: %v", err)
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000076_bno_predictive_capacity_v0_1b.up.sql")); err != nil {
		t.Fatalf("up: %v", err)
	}
	tenant := uuid.New()
	vehicle := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.vehicles (
			tenant_id, carrier_company_id, plate_number, vehicle_type, body_type, loading_access
		) VALUES ($1, $2, 'A123AA77', 'TRUCK', NULL, NULL)`, tenant, uuid.New()); err != nil {
		t.Fatalf("unknown capability insert: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.vehicles (
			tenant_id, carrier_company_id, plate_number, vehicle_type, loading_access
		) VALUES ($1, $2, 'B123AA77', 'TRUCK', '{}')`, tenant, uuid.New()); err != nil {
		t.Fatalf("known empty access insert: %v", err)
	}
	var loading *string
	if err := pool.QueryRow(ctx, `SELECT loading_access::text FROM transport.vehicles WHERE plate_number='A123AA77'`).Scan(&loading); err != nil || loading != nil {
		t.Fatalf("NULL access scanned as %v err=%v", loading, err)
	}
	capacityID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.capacities (
			id, owner_tenant_id, vehicle_id, location_label, available_from, available_until,
			source, visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,$3,'', now(), now() + interval '1 hour', 'CURRENT_SHIPMENT_PREDICTION', 'PRIVATE', 'PREDICTED', 1, now(), now())`,
		capacityID, tenant, vehicle); err != nil {
		t.Fatalf("predicted capacity insert: %v", err)
	}
	predictionID := uuid.New()
	shipmentID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.predicted_capacities (
			id, capacity_id, owner_tenant_id, shipment_id, shipment_version, shipment_status,
			vehicle_id, destination_location_id, eta, eta_observed_at, predicted_available_at,
			availability_window_start, availability_window_end, unload_duration_seconds,
			unload_policy_source, uncertainty_seconds, confidence, prediction_method,
			input_fingerprint, rule_version, generated_at, is_current, loading_access, version, created_at
		) VALUES (
			$1,$2,$3,$4,1,'IN_TRANSIT',$5,$6, now(), now(), now() + interval '2 hour',
			now() + interval '90 minute', now() + interval '150 minute', 2100,
			'BNO_PREDICTION_DEFAULT_UNLOAD_DURATION', 1200, 0.9, 'RULE_BASED',
			'fingerprint', 'bno-predict-0.1b.1', now(), true, NULL, 1, now()
		)`, predictionID, capacityID, tenant, shipmentID, vehicle, uuid.New()); err != nil {
		t.Fatalf("prediction insert: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.predicted_capacities (
			id, capacity_id, owner_tenant_id, shipment_id, shipment_version, shipment_status,
			vehicle_id, destination_location_id, eta, eta_observed_at, predicted_available_at,
			availability_window_start, availability_window_end, unload_duration_seconds,
			unload_policy_source, uncertainty_seconds, confidence, prediction_method,
			input_fingerprint, rule_version, generated_at, is_current, version, created_at
		) VALUES (
			$1,$2,$3,$4,1,'IN_TRANSIT',$5,$6, now(), now(), now() + interval '2 hour',
			now() + interval '90 minute', now() + interval '150 minute', 2100,
			'BNO_PREDICTION_DEFAULT_UNLOAD_DURATION', 1200, 0.9, 'RULE_BASED',
			'other', 'bno-predict-0.1b.1', now(), true, 1, now()
		)`, uuid.New(), capacityID, tenant, shipmentID, vehicle, uuid.New()); err == nil {
		t.Fatal("a second current prediction for the same shipment must fail")
	}
	sameShipment := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.predicted_capacities (
			id, capacity_id, owner_tenant_id, shipment_id, shipment_version, shipment_status,
			vehicle_id, destination_location_id, eta, eta_observed_at, predicted_available_at,
			availability_window_start, availability_window_end, unload_duration_seconds,
			unload_policy_source, uncertainty_seconds, confidence, prediction_method,
			input_fingerprint, rule_version, generated_at, is_current, version, created_at
		) VALUES (
			$1,$2,$3,$4,1,'IN_TRANSIT',$5,$6, now(), now(), now() + interval '2 hour',
			now() + interval '90 minute', now() + interval '150 minute', 2100,
			'BNO_PREDICTION_DEFAULT_UNLOAD_DURATION', 1200, 1.2, 'RULE_BASED',
			'bad', 'bno-predict-0.1b.1', now(), false, 1, now()
		)`, uuid.New(), capacityID, tenant, sameShipment, vehicle, uuid.New()); err == nil {
		t.Fatal("confidence above 1 must fail")
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000076_bno_predictive_capacity_v0_1b.down.sql")); err != nil {
		t.Fatalf("down: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.capacities (
			id, owner_tenant_id, location_label, available_from, available_until,
			source, visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'Yard', now(), now() + interval '1 hour', 'CURRENT_SHIPMENT_PREDICTION', 'PRIVATE', 'PREDICTED', 1, now(), now())`,
		uuid.New(), tenant); err == nil {
		t.Fatal("down migration must restore the manual-only source check")
	}
	if _, err := pool.Exec(ctx, mustRead(t, "000076_bno_predictive_capacity_v0_1b.up.sql")); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
