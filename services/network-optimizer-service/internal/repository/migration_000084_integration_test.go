//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO03E_Migration000084(t *testing.T) {
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
		"000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql",
		"000083_nlo_current_trip_fill_v0_3d.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	tenant := uuid.New()
	capacity := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.capacities (
			id, owner_tenant_id, location_label, available_from, available_until, source,
			visibility_scope, status, version, created_at, updated_at
		) VALUES ($1,$2,'Yard',now(),now() + interval '1 hour','MANUAL','PRIVATE','AVAILABLE',2,now(),now())`, capacity, tenant); err != nil {
		t.Fatal(err)
	}
	pairwiseID := uuid.New()
	currentID := uuid.New()
	insertLegacy := func() {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,$3,2,'SAME_ORIGIN_SAME_DESTINATION',now(),now(),'COMPLETED',2,1,now())`, pairwiseID, tenant, capacity); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,NULL,NULL,'CURRENT_TRIP_FILL',now(),now(),'COMPLETED',1,1,now())`, currentID, tenant); err != nil {
			t.Fatal(err)
		}
	}
	insertLegacy()
	if _, err := pool.Exec(ctx, mustRead(t, "000084_nlo_bounded_n_member_search_v0_3e.up.sql")); err != nil {
		t.Fatal(err)
	}
	var pairCount *int
	var setCount *int
	if err := pool.QueryRow(ctx, `SELECT evaluated_pair_count, evaluated_set_count FROM network_optimizer.consolidation_search_runs WHERE id=$1`, pairwiseID).Scan(&pairCount, &setCount); err != nil || pairCount == nil || *pairCount != 1 || setCount != nil {
		t.Fatalf("pairwise survive pair %v set %v err %v", pairCount, setCount, err)
	}
	if err := pool.QueryRow(ctx, `SELECT evaluated_pair_count, evaluated_set_count FROM network_optimizer.consolidation_search_runs WHERE id=$1`, currentID).Scan(&pairCount, &setCount); err != nil || pairCount == nil || setCount != nil {
		t.Fatalf("current survive pair %v set %v err %v", pairCount, setCount, err)
	}

	nMemberID := uuid.New()
	insertNMember := func(pair any, set any) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, evaluated_set_count, created_at
			) VALUES ($1,$2,$3,2,'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER',now(),now(),'COMPLETED',2,$4,$5,now())`,
			uuid.New(), tenant, capacity, pair, set)
		return err
	}
	t.Run("N_MEMBER_PATTERN_ACCEPTED_BY_DB", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, evaluated_set_count, created_at
			) VALUES ($1,$2,$3,2,'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER',now(),now(),'COMPLETED',2,NULL,1,now())`,
			nMemberID, tenant, capacity); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("UNKNOWN_PATTERN_REJECTED_BY_DB", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, evaluated_set_count, created_at
			) VALUES ($1,$2,$3,2,'OTHER',now(),now(),'COMPLETED',0,0,NULL,now())`, uuid.New(), tenant, capacity); err == nil {
			t.Fatal("unknown pattern accepted")
		}
	})
	t.Run("PAIRWISE_REQUIRES_CAPACITY", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,NULL,NULL,'SAME_ORIGIN_SAME_DESTINATION',now(),now(),'COMPLETED',0,0,now())`, uuid.New(), tenant); err == nil {
			t.Fatal("pairwise without capacity accepted")
		}
	})
	t.Run("N_MEMBER_REQUIRES_CAPACITY", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, pattern, started_at, completed_at, status, pool_load_count,
				evaluated_pair_count, evaluated_set_count, created_at
			) VALUES ($1,$2,'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER',now(),now(),'COMPLETED',0,NULL,0,now())`, uuid.New(), tenant); err == nil {
			t.Fatal("n-member without capacity accepted")
		}
	})
	t.Run("CURRENT_TRIP_REQUIRES_NO_CAPACITY", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, created_at
			) VALUES ($1,$2,$3,2,'CURRENT_TRIP_FILL',now(),now(),'COMPLETED',0,0,now())`, uuid.New(), tenant, capacity); err == nil {
			t.Fatal("current trip with capacity accepted")
		}
	})
	t.Run("PAIRWISE_PAIR_COUNT_REQUIRED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, evaluated_set_count, created_at
			) VALUES ($1,$2,$3,2,'SAME_ORIGIN_SAME_DESTINATION',now(),now(),'COMPLETED',0,NULL,NULL,now())`, uuid.New(), tenant, capacity); err == nil {
			t.Fatal("pairwise null pair count accepted")
		}
	})
	t.Run("PAIRWISE_SET_COUNT_NULL", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, evaluated_set_count, created_at
			) VALUES ($1,$2,$3,2,'SAME_ORIGIN_SAME_DESTINATION',now(),now(),'COMPLETED',0,1,1,now())`, uuid.New(), tenant, capacity); err == nil {
			t.Fatal("pairwise set count accepted")
		}
	})
	t.Run("N_MEMBER_SET_COUNT_REQUIRED", func(t *testing.T) {
		if err := insertNMember(nil, nil); err == nil {
			t.Fatal("n-member null set count accepted")
		}
	})
	t.Run("N_MEMBER_PAIR_COUNT_NULL", func(t *testing.T) {
		if err := insertNMember(1, 1); err == nil {
			t.Fatal("n-member pair count accepted")
		}
	})
	candidateID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.consolidation_candidates (
			id, search_run_id, tenant_id, capacity_id, pattern, status, execution_supported,
			compatibility_status, compatibility_fingerprint, candidate_fingerprint, placement_check, created_at
		) VALUES ($1,$2,$3,$4,'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER','FEASIBLE',false,'COMPATIBLE','fp','cand-fp','NOT_EVALUATED',now())`,
		candidateID, nMemberID, tenant, capacity); err != nil {
		t.Fatal(err)
	}
	loadA, loadB, loadC := uuid.New(), uuid.New(), uuid.New()
	insertMember := func(ordinal int, load uuid.UUID) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_candidate_members (
				candidate_id, ordinal, load_opportunity_id, load_version, load_owner_tenant_id, created_at
			) VALUES ($1,$2,$3,1,$4,now())`, candidateID, ordinal, load, tenant)
		return err
	}
	if err := insertMember(1, loadA); err != nil {
		t.Fatal(err)
	}
	if err := insertMember(2, loadB); err != nil {
		t.Fatal(err)
	}
	t.Run("N_MEMBER_PATTERN_AND_TWO_MEMBERS_STILL_N_MEMBER_MODE", func(t *testing.T) {
		var pattern string
		if err := pool.QueryRow(ctx, `SELECT pattern FROM network_optimizer.consolidation_candidates WHERE id=$1`, candidateID).Scan(&pattern); err != nil || pattern != "SAME_ORIGIN_SAME_DESTINATION_N_MEMBER" {
			t.Fatalf("%s %v", pattern, err)
		}
	})
	t.Run("N_MEMBER_ORDINAL_3_ACCEPTED", func(t *testing.T) {
		if err := insertMember(3, loadC); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("N_MEMBER_ORDINAL_4_REJECTED", func(t *testing.T) {
		if err := insertMember(4, uuid.New()); err == nil {
			t.Fatal("ordinal 4 accepted")
		}
	})
	t.Run("PAIRWISE_EXISTING_ROWS_SURVIVE_UP", func(t *testing.T) {
		var pattern string
		if err := pool.QueryRow(ctx, `SELECT pattern FROM network_optimizer.consolidation_search_runs WHERE id=$1`, pairwiseID).Scan(&pattern); err != nil || pattern != "SAME_ORIGIN_SAME_DESTINATION" {
			t.Fatal(err)
		}
	})
	t.Run("CURRENT_TRIP_EXISTING_ROWS_SURVIVE_UP", func(t *testing.T) {
		var pattern string
		if err := pool.QueryRow(ctx, `SELECT pattern FROM network_optimizer.consolidation_search_runs WHERE id=$1`, currentID).Scan(&pattern); err != nil || pattern != "CURRENT_TRIP_FILL" {
			t.Fatal(err)
		}
	})
	t.Run("DOWN_WITH_N_MEMBER_ROWS_FAILS_CLOSED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000084_nlo_bounded_n_member_search_v0_3e.down.sql")); err == nil {
			t.Fatal("down accepted n-member rows")
		}
		var pattern string
		if err := pool.QueryRow(ctx, `SELECT pattern FROM network_optimizer.consolidation_search_runs WHERE id=$1`, nMemberID).Scan(&pattern); err != nil || pattern != "SAME_ORIGIN_SAME_DESTINATION_N_MEMBER" {
			t.Fatalf("row changed %s %v", pattern, err)
		}
	})
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.consolidation_candidate_members WHERE candidate_id=$1`, candidateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.consolidation_candidates WHERE id=$1`, candidateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.consolidation_search_runs WHERE id=$1`, nMemberID); err != nil {
		t.Fatal(err)
	}
	ordinalCandidate := uuid.New()
	ordinalRun := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.consolidation_search_runs (
			id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
			status, pool_load_count, evaluated_pair_count, created_at
		) VALUES ($1,$2,$3,2,'SAME_ORIGIN_SAME_DESTINATION',now(),now(),'COMPLETED',1,1,now())`, ordinalRun, tenant, capacity); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.consolidation_candidates (
			id, search_run_id, tenant_id, capacity_id, pattern, status, execution_supported,
			compatibility_status, compatibility_fingerprint, candidate_fingerprint, placement_check, created_at
		) VALUES ($1,$2,$3,$4,'SAME_ORIGIN_SAME_DESTINATION','FEASIBLE',false,'COMPATIBLE','fp2','cand-fp2','NOT_EVALUATED',now())`,
		ordinalCandidate, ordinalRun, tenant, capacity); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.consolidation_candidate_members (
			candidate_id, ordinal, load_opportunity_id, load_version, load_owner_tenant_id, created_at
		) VALUES ($1,3,$2,1,$3,now())`, ordinalCandidate, uuid.New(), tenant); err != nil {
		t.Fatal(err)
	}
	t.Run("DOWN_WITH_ORDINAL_3_FAILS_CLOSED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000084_nlo_bounded_n_member_search_v0_3e.down.sql")); err == nil {
			t.Fatal("down accepted ordinal 3")
		}
		var ordinal int
		if err := pool.QueryRow(ctx, `SELECT ordinal FROM network_optimizer.consolidation_candidate_members WHERE candidate_id=$1`, ordinalCandidate).Scan(&ordinal); err != nil || ordinal != 3 {
			t.Fatalf("ordinal %d err %v", ordinal, err)
		}
	})
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.consolidation_candidate_members WHERE candidate_id=$1`, ordinalCandidate); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.consolidation_candidates WHERE id=$1`, ordinalCandidate); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.consolidation_search_runs WHERE id=$1`, ordinalRun); err != nil {
		t.Fatal(err)
	}
	t.Run("DOWN_CLEAN", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000084_nlo_bounded_n_member_search_v0_3e.down.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_search_runs (
				id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at,
				status, pool_load_count, evaluated_pair_count, evaluated_set_count, created_at
			) VALUES ($1,$2,$3,2,'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER',now(),now(),'COMPLETED',0,NULL,0,now())`, uuid.New(), tenant, capacity); err == nil {
			t.Fatal("n-member accepted after down")
		}
	})
	t.Run("UP_DOWN_UP", func(t *testing.T) {
		if _, err := pool.Exec(ctx, mustRead(t, "000084_nlo_bounded_n_member_search_v0_3e.up.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000084_nlo_bounded_n_member_search_v0_3e.down.sql")); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, mustRead(t, "000084_nlo_bounded_n_member_search_v0_3e.up.sql")); err != nil {
			t.Fatal(err)
		}
		if err := insertNMember(nil, 2); err != nil {
			t.Fatal(err)
		}
	})
}
