//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func TestBNO316_BNO317_BNO318_BNO319_BNO320(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000076(t, ctx, pool)
	for _, name := range []string{
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
		"000080_bno_match_score_topn_v0_1c2.up.sql",
	} {
		if _, err := pool.Exec(ctx, mustRead(t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	store := NewPostgres(pool)

	t.Run("BNO316_POSTGRES_PROFILE_LOADED_FROM_DB", func(t *testing.T) {
		rows, err := pool.Query(ctx, `
			SELECT c.component_code, c.weight_bps, c.required, c.ordinal
			FROM network_optimizer.score_profile_components c
			JOIN network_optimizer.score_profiles p ON p.id = c.profile_id
			WHERE p.code='BALANCED' AND p.scope='SYSTEM' AND p.status='ACTIVE' AND p.tenant_id IS NULL
			ORDER BY c.ordinal, c.component_code`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var persisted []domain.ScoreProfileComponent
		for rows.Next() {
			var component domain.ScoreProfileComponent
			if err := rows.Scan(&component.Code, &component.WeightBps, &component.Required, &component.Ordinal); err != nil {
				t.Fatal(err)
			}
			persisted = append(persisted, component)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		profile, err := store.ActiveScoreProfile(ctx, domain.ProfileBalanced)
		if err != nil {
			t.Fatal(err)
		}
		if err := domain.ValidateActiveScoreProfile(profile); err != nil {
			t.Fatal(err)
		}
		if len(profile.Components) != len(persisted) || len(persisted) == 0 {
			t.Fatalf("loaded %d persisted %d", len(profile.Components), len(persisted))
		}
		for i := range persisted {
			if profile.Components[i] != persisted[i] {
				t.Fatalf("component %d loaded %+v persisted %+v", i, profile.Components[i], persisted[i])
			}
		}
	})

	t.Run("BNO319_PROFILE_ORDINAL_UNIQUE", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO network_optimizer.score_profile_components (profile_id, component_code, weight_bps, required, ordinal)
			SELECT id, 'NETWORK_VALUE', 1, false, 1
			FROM network_optimizer.score_profiles
			WHERE code='MIN_DEADHEAD'`)
		if !uniqueViolation(err) {
			t.Fatalf("duplicate ordinal inserted: %v", err)
		}
	})

	t.Run("BNO320_PROFILE_COMPONENT_VALIDATION", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			UPDATE network_optimizer.score_profile_components
			SET component_code='NOT_A_COMPONENT'
			WHERE profile_id = (SELECT id FROM network_optimizer.score_profiles WHERE code='MAX_REVENUE')
			  AND component_code='REVENUE'`); err != nil {
			t.Fatal(err)
		}
		if _, err := store.ActiveScoreProfile(ctx, domain.ProfileMaxRevenue); !errors.Is(err, domain.ErrScoreProfileInvalid) {
			t.Fatalf("unknown component accepted: %v", err)
		}
	})

	t.Run("BNO317_INVALID_PROFILE_WEIGHT_SUM_REJECTED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			UPDATE network_optimizer.score_profile_components
			SET weight_bps = 3400
			WHERE profile_id = (SELECT id FROM network_optimizer.score_profiles WHERE code='BALANCED')
			  AND component_code='DEADHEAD_EFFICIENCY'`); err != nil {
			t.Fatal(err)
		}
		if _, err := store.ActiveScoreProfile(ctx, domain.ProfileBalanced); !errors.Is(err, domain.ErrScoreProfileInvalid) {
			t.Fatalf("corrupt weight sum accepted: %v", err)
		}
	})

	t.Run("BNO318_UNSUPPORTED_ALGORITHM_VERSION_REJECTED", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			UPDATE network_optimizer.score_profiles
			SET algorithm_version='bno-score-unsupported'
			WHERE code='RETURN_HOME'`); err != nil {
			t.Fatal(err)
		}
		profile, err := store.ActiveScoreProfile(ctx, domain.ProfileReturnHome)
		if !errors.Is(err, domain.ErrScoringAlgorithmUnsupported) {
			t.Fatalf("unsupported algorithm accepted: %v", err)
		}
		if profile.AlgorithmVersion != "" {
			t.Fatalf("unsupported profile returned %+v", profile)
		}
		var claimed int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM network_optimizer.next_load_search_runs
			WHERE scoring_algorithm_version='bno-score-unsupported'`).Scan(&claimed); err != nil || claimed != 0 {
			t.Fatalf("unsupported version was recorded %d %v", claimed, err)
		}
	})
}
