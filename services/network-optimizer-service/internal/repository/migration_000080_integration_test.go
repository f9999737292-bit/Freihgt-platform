//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBNO300_BNO301_BNO302_BNO303_BNO312_BNO313_BNO314_BNO315(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	for _, name := range []string{
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	carrier := uuid.New()
	shipper := uuid.New()
	capacityID := uuid.New()
	loadID := uuid.New()
	runID := uuid.New()
	eligibleID := uuid.New()
	rejectedID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.capacities (
			id, owner_tenant_id, location_label, available_from, available_until,
			source, visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'', now(), now() + interval '2 hours', 'MANUAL', 'PRIVATE', 'AVAILABLE', 1, now(), now())`,
		capacityID, carrier); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.load_opportunities (
			id, owner_tenant_id, source_type, source_id, visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'TRANSPORT_ORDER',$3,'MARKETPLACE','PUBLISHED',1,now(),now())`,
		loadID, shipper, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.next_load_search_runs (
			id, tenant_id, capacity_id, capacity_version, effective_policy_fingerprint,
			started_at, completed_at, routing_provider, status
		) VALUES ($1,$2,$3,1,'fp', now(), now(), 'UNSPECIFIED', 'COMPLETED')`,
		runID, carrier, capacityID); err != nil {
		t.Fatal(err)
	}
	insertCandidate := func(id, eligibility string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.match_candidates (
				id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
				eligibility, compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at
			) VALUES ($1,$2,$3,$4,$5,1,$6,'COMPATIBLE','fp','fp', now())`,
			id, runID, carrier, capacityID, loadID, eligibility); err != nil {
			t.Fatal(err)
		}
	}
	insertCandidate(eligibleID.String(), "ELIGIBLE")
	insertCandidate(rejectedID.String(), "REJECTED")

	t.Run("BNO312_MIGRATION_000080_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000080_bno_match_score_topn_v0_1c2.up.sql")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("BNO315_EXISTING_C1_ROWS_COMPATIBLE", func(t *testing.T) {
		var eligibleStatus, rejectedStatus string
		var eligibleScore, rejectedScore *int
		var eligibleRank, rejectedRank *int
		if err := pool.QueryRow(ctx, `SELECT score_status, score_total, rank FROM network_optimizer.match_candidates WHERE id=$1`, eligibleID).Scan(&eligibleStatus, &eligibleScore, &eligibleRank); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT score_status, score_total, rank FROM network_optimizer.match_candidates WHERE id=$1`, rejectedID).Scan(&rejectedStatus, &rejectedScore, &rejectedRank); err != nil {
			t.Fatal(err)
		}
		if eligibleStatus != "UNRANKED" || eligibleScore != nil || eligibleRank != nil || rejectedStatus != "NOT_APPLICABLE" || rejectedScore != nil || rejectedRank != nil {
			t.Fatalf("eligible %s rejected %s", eligibleStatus, rejectedStatus)
		}
		var active int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM network_optimizer.score_profiles WHERE status='ACTIVE'`).Scan(&active); err != nil || active != 6 {
			t.Fatalf("active %d %v", active, err)
		}
	})
	t.Run("BNO300_REJECTED_SCORE_NULL_DB_CONSTRAINT", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			UPDATE network_optimizer.match_candidates SET score_total=10 WHERE id=$1`, rejectedID)
		if !checkViolation(err) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("BNO301_UNRANKED_SCORE_NULL_DB_CONSTRAINT", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			UPDATE network_optimizer.match_candidates SET score_total=10 WHERE id=$1`, eligibleID)
		if !checkViolation(err) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("BNO303_SCORE_RANGE_DB_CONSTRAINT", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			UPDATE network_optimizer.match_candidates
			SET score_status='RANKED', rank=1, score_total=10001, score_evidence_bps=10000
			WHERE id=$1`, eligibleID)
		if !checkViolation(err) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("BNO302_RANK_UNIQUE_PER_SEARCH_RUN", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			UPDATE network_optimizer.match_candidates
			SET score_status='RANKED', rank=1, score_total=1000, score_evidence_bps=10000
			WHERE id=$1`, eligibleID); err != nil {
			t.Fatal(err)
		}
		secondLoad := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.load_opportunities (
				id, owner_tenant_id, source_type, source_id, visibility_scope, status, version, created_at, updated_at
			) VALUES ($1,$2,'TRANSPORT_ORDER',$3,'MARKETPLACE','PUBLISHED',1,now(),now())`,
			secondLoad, shipper, uuid.New()); err != nil {
			t.Fatal(err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.match_candidates (
				id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
				eligibility, compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at,
				score_status, rank, score_total, score_evidence_bps
			) VALUES ($1,$2,$3,$4,$5,1,'ELIGIBLE','COMPATIBLE','fp','fp', now(), 'RANKED', 1, 900, 10000)`,
			uuid.New(), runID, carrier, capacityID, secondLoad)
		if !uniqueViolation(err) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("BNO313_MIGRATION_000080_DOWN", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000080_bno_match_score_topn_v0_1c2.down.sql")); err != nil {
			t.Fatal(err)
		}
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='network_optimizer' AND table_name='score_profiles')`).Scan(&exists); err != nil || exists {
			t.Fatalf("profiles remain %v %v", exists, err)
		}
	})
	t.Run("BNO314_MIGRATION_000080_UP_DOWN_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000080_bno_match_score_topn_v0_1c2.up.sql")); err != nil {
			t.Fatal(err)
		}
		var weight int
		if err := pool.QueryRow(ctx, `
			SELECT COALESCE(sum(weight_bps), 0)
			FROM network_optimizer.score_profile_components c
			JOIN network_optimizer.score_profiles p ON p.id = c.profile_id
			WHERE p.code='BALANCED' AND p.status='ACTIVE'`).Scan(&weight); err != nil || weight != 10000 {
			t.Fatalf("balanced %d %v", weight, err)
		}
	})
}

func checkViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
