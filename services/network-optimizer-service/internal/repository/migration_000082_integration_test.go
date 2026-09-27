//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO03C_001_003_Migration000082(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	for _, name := range []string{
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
		"000080_bno_match_score_topn_v0_1c2.up.sql",
		"000081_nlo_pairwise_consolidation_v0_3b.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := uuid.New()
	origin := uuid.New()
	destination := uuid.New()
	cargo := uuid.New()
	shipment := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code)
		VALUES ($1,$2,'WAREHOUSE','Origin','RU'), ($3,$2,'WAREHOUSE','Destination','RU')`,
		origin, tenant, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type) VALUES ($1,$2,'GENERAL')`, cargo, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, status
		) VALUES ($1,$2,'SH-082',$3,$3,$4,$5,$6,'LOADED')`,
		shipment, tenant, uuid.New(), origin, destination, cargo); err != nil {
		t.Fatal(err)
	}

	t.Run("NLO03C_001_MIGRATION_000082_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO transport.shipment_cargo_execution_evidence (
				id, tenant_id, shipment_id, shipment_version, cargo_id, state, state_version,
				source, source_event_type, occurred_at
			) VALUES ($1,$2,$3,2,$4,'CONFIRMED_ONBOARD',2,'DRIVER_OPERATION','PICKUP_COMPLETED',now())`,
			uuid.New(), tenant, shipment, cargo); err != nil {
			t.Fatal(err)
		}
		var rows int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM transport.shipment_cargo_execution_evidence WHERE shipment_id=$1`, shipment).Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("rows %d err %v", rows, err)
		}
	})
	t.Run("NLO03C_004_NO_HISTORICAL_LOADED_BACKFILL", func(t *testing.T) {
		var rows int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM transport.shipment_cargo_execution_evidence`).Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("backfill rows %d err %v", rows, err)
		}
	})
	t.Run("NLO03C_002_MIGRATION_000082_DOWN", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000082_nlo_onboard_evidence_current_trip_context_v0_3c.down.sql")); err != nil {
			t.Fatal(err)
		}
		var evidence, consolidation bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='transport' AND table_name='shipment_cargo_execution_evidence')`).Scan(&evidence); err != nil || evidence {
			t.Fatalf("evidence remains %v %v", evidence, err)
		}
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='consolidation_search_runs')`).Scan(&consolidation); err != nil || !consolidation {
			t.Fatalf("000081 removed %v %v", consolidation, err)
		}
	})
	t.Run("NLO03C_003_MIGRATION_000082_UP_DOWN_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000082_nlo_onboard_evidence_current_trip_context_v0_3c.down.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql")); err != nil {
			t.Fatal(err)
		}
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='transport' AND table_name='shipment_cargo_execution_evidence')`).Scan(&exists); err != nil || !exists {
			t.Fatalf("reapplied %v %v", exists, err)
		}
	})
}
