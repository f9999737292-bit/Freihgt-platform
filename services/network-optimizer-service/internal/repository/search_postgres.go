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
			started_at, completed_at, routing_provider, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		run.ID, run.TenantID, run.CapacityID, run.CapacityVersion, run.EffectivePolicyFingerprint,
		run.StartedAt, run.CompletedAt, run.RoutingProvider, run.Status); err != nil {
		return err
	}
	for _, candidate := range candidates {
		if _, err := tx.Exec(ctx, `
			INSERT INTO network_optimizer.match_candidates (
				id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
				eligibility, reject_reasons, road_deadhead_km, road_deadhead_minutes, waiting_minutes,
				compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			candidate.ID, candidate.SearchRunID, candidate.TenantID, candidate.CapacityID, candidate.LoadOpportunityID,
			candidate.LoadVersion, candidate.Eligibility, candidate.RejectReasons, candidate.RoadDeadheadKm,
			candidate.RoadDeadheadMinutes, candidate.WaitingMinutes, candidate.CompatibilityStatus,
			candidate.CompatibilityFingerprint, candidate.PolicyFingerprint, candidate.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) GetSearch(ctx context.Context, tenant, id uuid.UUID) (SearchRun, []StoredCandidate, error) {
	var run SearchRun
	err := p.pool.QueryRow(ctx, `
		SELECT id, tenant_id, capacity_id, capacity_version, effective_policy_fingerprint,
			started_at, completed_at, routing_provider, status
		FROM network_optimizer.next_load_search_runs WHERE id=$1 AND tenant_id=$2`, id, tenant).Scan(
		&run.ID, &run.TenantID, &run.CapacityID, &run.CapacityVersion, &run.EffectivePolicyFingerprint,
		&run.StartedAt, &run.CompletedAt, &run.RoutingProvider, &run.Status)
	if err != nil {
		return SearchRun{}, nil, ErrNotFound
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, search_run_id, tenant_id, capacity_id, load_opportunity_id, load_version,
			eligibility, reject_reasons, road_deadhead_km, road_deadhead_minutes, waiting_minutes,
			compatibility_status, compatibility_fingerprint, policy_fingerprint, created_at
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
			&candidate.CreatedAt); err != nil {
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
			predicted_available_at, availability_window_start, availability_window_end, is_current
		FROM network_optimizer.predicted_capacities
		WHERE owner_tenant_id=$1 AND capacity_id=$2 AND is_current
		ORDER BY generated_at DESC LIMIT 1`, tenant, capacityID).Scan(
		&prediction.ID, &prediction.CapacityID, &prediction.CombinationType, &prediction.BodyType,
		&prediction.LoadingAccess, &prediction.UnloadingAccess, &prediction.CapacityWeightKg, &prediction.CapacityVolumeM3,
		&prediction.TemperatureControlMode, &prediction.TemperatureCapabilityMinC, &prediction.TemperatureCapabilityMaxC,
		&prediction.TemperatureZoneCount, &prediction.IndependentTemperatureControl, &prediction.LegacyEquipmentType,
		&prediction.ContainerSize, &prediction.PredictedAvailableAt, &prediction.AvailabilityWindowStart,
		&prediction.AvailabilityWindowEnd, &prediction.IsCurrent)
	if err != nil {
		return domain.PredictedCapacity{}, ErrNotFound
	}
	prediction.OwnerTenantID = tenant
	return prediction, nil
}
