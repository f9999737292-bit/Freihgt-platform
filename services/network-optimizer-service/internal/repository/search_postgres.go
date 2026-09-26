package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func (p *Postgres) SaveSearch(ctx context.Context, run SearchRun, candidates []StoredCandidate) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO network_optimizer.next_load_search_runs (
			id, tenant_id, capacity_id, capacity_version, effective_policy_fingerprint,
			started_at, completed_at, routing_provider, status,
			score_profile_code, score_profile_version, score_profile_fingerprint,
			scoring_algorithm_version, ranking_currency
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		run.ID, run.TenantID, run.CapacityID, run.CapacityVersion, run.EffectivePolicyFingerprint,
		run.StartedAt, run.CompletedAt, run.RoutingProvider, run.Status,
		run.ScoreProfileCode, run.ScoreProfileVersion, run.ScoreProfileFingerprint,
		run.ScoringAlgorithmVersion, run.RankingCurrency); err != nil {
		return err
	}
	for _, candidate := range candidates {
		if _, err := tx.Exec(ctx, `
			INSERT INTO network_optimizer.match_candidates (
				id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
				eligibility, reject_reasons, road_deadhead_km, road_deadhead_minutes, waiting_minutes,
				compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at,
				rank, score_status, score_total, score_evidence_bps, score_fingerprint,
				score_components, unranked_reason_codes
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21::jsonb,$22)`,
			candidate.ID, candidate.SearchRunID, candidate.TenantID, candidate.CapacityID, candidate.LoadOpportunityID,
			candidate.LoadVersion, candidate.Eligibility, candidate.RejectReasons, candidate.RoadDeadheadKm,
			candidate.RoadDeadheadMinutes, candidate.WaitingMinutes, candidate.CompatibilityStatus,
			candidate.CompatibilityFingerprint, candidate.PolicyFingerprint, candidate.CreatedAt,
			candidate.Rank, candidate.ScoreStatus, candidate.ScoreTotal, candidate.ScoreEvidenceBps,
			candidate.ScoreFingerprint, candidate.ScoreComponents, candidate.UnrankedReasonCodes); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) GetSearch(ctx context.Context, tenant, id uuid.UUID) (SearchRun, []StoredCandidate, error) {
	var run SearchRun
	err := p.pool.QueryRow(ctx, `
		SELECT id, tenant_id, capacity_id, capacity_version, effective_policy_fingerprint,
			started_at, completed_at, routing_provider, status,
			COALESCE(score_profile_code, ''), COALESCE(score_profile_version, 0),
			COALESCE(score_profile_fingerprint, ''), COALESCE(scoring_algorithm_version, ''),
			ranking_currency
		FROM network_optimizer.next_load_search_runs WHERE id=$1 AND tenant_id=$2`, id, tenant).Scan(
		&run.ID, &run.TenantID, &run.CapacityID, &run.CapacityVersion, &run.EffectivePolicyFingerprint,
		&run.StartedAt, &run.CompletedAt, &run.RoutingProvider, &run.Status,
		&run.ScoreProfileCode, &run.ScoreProfileVersion, &run.ScoreProfileFingerprint,
		&run.ScoringAlgorithmVersion, &run.RankingCurrency)
	if err != nil {
		return SearchRun{}, nil, ErrNotFound
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
			eligibility, reject_reasons, road_deadhead_km, road_deadhead_minutes, waiting_minutes,
			compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at,
			rank, score_status, score_total, score_evidence_bps, score_fingerprint,
			score_components, unranked_reason_codes
		FROM network_optimizer.match_candidates WHERE search_run_id=$1 AND tenant_id=$2 ORDER BY load_opportunity_id`, id, tenant)
	if err != nil {
		return SearchRun{}, nil, err
	}
	defer rows.Close()
	var candidates []StoredCandidate
	for rows.Next() {
		var candidate StoredCandidate
		if err := rows.Scan(&candidate.ID, &candidate.SearchRunID, &candidate.TenantID, &candidate.CapacityID,
			&candidate.LoadOpportunityID, &candidate.LoadVersion, &candidate.Eligibility, &candidate.RejectReasons,
			&candidate.RoadDeadheadKm, &candidate.RoadDeadheadMinutes, &candidate.WaitingMinutes,
			&candidate.CompatibilityStatus, &candidate.CompatibilityFingerprint, &candidate.PolicyFingerprint,
			&candidate.CreatedAt, &candidate.Rank, &candidate.ScoreStatus, &candidate.ScoreTotal,
			&candidate.ScoreEvidenceBps, &candidate.ScoreFingerprint, &candidate.ScoreComponents,
			&candidate.UnrankedReasonCodes); err != nil {
			return SearchRun{}, nil, err
		}
		candidates = append(candidates, candidate)
	}
	return run, candidates, rows.Err()
}

func (p *Postgres) CurrentPredictionForCapacity(ctx context.Context, tenant, capacityID uuid.UUID) (domain.PredictedCapacity, error) {
	var prediction domain.PredictedCapacity
	err := p.pool.QueryRow(ctx, `
		SELECT id, capacity_id, combination_type, body_type, loading_access, unloading_access,
			capacity_weight_kg, capacity_volume_m3, temperature_control_mode,
			temperature_capability_min_c, temperature_capability_max_c, temperature_zone_count,
			independent_temperature_control, legacy_equipment_type, container_size,
			predicted_available_at, availability_window_start, availability_window_end, is_current,
			confidence, uncertainty_seconds
		FROM network_optimizer.predicted_capacities
		WHERE owner_tenant_id=$1 AND capacity_id=$2 AND is_current
		ORDER BY generated_at DESC LIMIT 1`, tenant, capacityID).Scan(
		&prediction.ID, &prediction.CapacityID, &prediction.CombinationType, &prediction.BodyType,
		&prediction.LoadingAccess, &prediction.UnloadingAccess, &prediction.CapacityWeightKg, &prediction.CapacityVolumeM3,
		&prediction.TemperatureControlMode, &prediction.TemperatureCapabilityMinC, &prediction.TemperatureCapabilityMaxC,
		&prediction.TemperatureZoneCount, &prediction.IndependentTemperatureControl, &prediction.LegacyEquipmentType,
		&prediction.ContainerSize, &prediction.PredictedAvailableAt, &prediction.AvailabilityWindowStart,
		&prediction.AvailabilityWindowEnd, &prediction.IsCurrent, &prediction.Confidence, &prediction.UncertaintySeconds)
	if err != nil {
		return domain.PredictedCapacity{}, ErrNotFound
	}
	prediction.OwnerTenantID = tenant
	return prediction, nil
}
