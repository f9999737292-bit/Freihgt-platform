package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

type PolicyStore interface {
	UpsertCarrierPolicy(ctx context.Context, tenant uuid.UUID, policy domain.NextLoadSearchPolicy) error
	GetCarrierPolicy(ctx context.Context, tenant uuid.UUID) (domain.NextLoadSearchPolicy, error)
	UpsertCapacityPolicy(ctx context.Context, tenant, capacityID uuid.UUID, policy domain.NextLoadSearchPolicy) error
	GetCapacityPolicy(ctx context.Context, tenant, capacityID uuid.UUID) (domain.NextLoadSearchPolicy, error)
}

func (m *Memory) UpsertCarrierPolicy(_ context.Context, tenant uuid.UUID, policy domain.NextLoadSearchPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.carrierPolicies == nil {
		m.carrierPolicies = map[uuid.UUID]domain.NextLoadSearchPolicy{}
	}
	m.carrierPolicies[tenant] = policy
	return nil
}

func (m *Memory) GetCarrierPolicy(_ context.Context, tenant uuid.UUID) (domain.NextLoadSearchPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy, ok := m.carrierPolicies[tenant]
	if !ok {
		return domain.NextLoadSearchPolicy{}, ErrNotFound
	}
	return policy, nil
}

func (m *Memory) UpsertCapacityPolicy(_ context.Context, tenant, capacityID uuid.UUID, policy domain.NextLoadSearchPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	capacity, ok := m.caps[capacityID]
	if !ok || capacity.OwnerTenantID != tenant {
		return ErrNotFound
	}
	if existing, exists := m.capacityPolicies[capacityID]; exists && existing.TenantID != tenant {
		return ErrNotFound
	}
	if m.capacityPolicies == nil {
		m.capacityPolicies = map[uuid.UUID]storedCapacityPolicy{}
	}
	m.capacityPolicies[capacityID] = storedCapacityPolicy{TenantID: tenant, Policy: policy}
	return nil
}

func (m *Memory) GetCapacityPolicy(_ context.Context, tenant, capacityID uuid.UUID) (domain.NextLoadSearchPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.capacityPolicies[capacityID]
	if !ok || stored.TenantID != tenant {
		return domain.NextLoadSearchPolicy{}, ErrNotFound
	}
	return stored.Policy, nil
}

type storedCapacityPolicy struct {
	TenantID uuid.UUID
	Policy   domain.NextLoadSearchPolicy
}

func (p *Postgres) UpsertCarrierPolicy(ctx context.Context, tenant uuid.UUID, policy domain.NextLoadSearchPolicy) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO network_optimizer.carrier_search_policies (
			owner_tenant_id, search_mode, target_location_id, forward_search_km, corridor_deviation_km,
			max_deadhead_km, preferred_deadhead_km, max_deadhead_minutes, min_loaded_distance_km,
			max_route_increase_km, objective_profile, allow_unknown_road_distance, version, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,1,now())
		ON CONFLICT (owner_tenant_id) DO UPDATE SET
			search_mode=EXCLUDED.search_mode, target_location_id=EXCLUDED.target_location_id,
			forward_search_km=EXCLUDED.forward_search_km, corridor_deviation_km=EXCLUDED.corridor_deviation_km,
			max_deadhead_km=EXCLUDED.max_deadhead_km, preferred_deadhead_km=EXCLUDED.preferred_deadhead_km,
			max_deadhead_minutes=EXCLUDED.max_deadhead_minutes, min_loaded_distance_km=EXCLUDED.min_loaded_distance_km,
			max_route_increase_km=EXCLUDED.max_route_increase_km, objective_profile=EXCLUDED.objective_profile,
			allow_unknown_road_distance=EXCLUDED.allow_unknown_road_distance, version=carrier_search_policies.version+1,
			updated_at=now()`,
		tenant, policy.SearchMode, policy.TargetLocationID, policy.ForwardSearchKm, policy.CorridorDeviationKm,
		policy.MaxDeadheadKm, policy.PreferredDeadheadKm, policy.MaxDeadheadMinutes, policy.MinLoadedDistanceKm,
		policy.MaxRouteIncreaseKm, policy.ObjectiveProfile, policy.AllowUnknownRoadDistance,
	)
	return err
}

func (p *Postgres) GetCarrierPolicy(ctx context.Context, tenant uuid.UUID) (domain.NextLoadSearchPolicy, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT search_mode, target_location_id, forward_search_km, corridor_deviation_km,
			max_deadhead_km, preferred_deadhead_km, max_deadhead_minutes, min_loaded_distance_km,
			max_route_increase_km, objective_profile, allow_unknown_road_distance
		FROM network_optimizer.carrier_search_policies WHERE owner_tenant_id=$1`, tenant)
	return scanPolicy(row)
}

func (p *Postgres) UpsertCapacityPolicy(ctx context.Context, tenant, capacityID uuid.UUID, policy domain.NextLoadSearchPolicy) error {
	tag, err := p.pool.Exec(ctx, `
		WITH owned AS (
			SELECT id, owner_tenant_id
			FROM network_optimizer.capacities
			WHERE id = $1 AND owner_tenant_id = $2
		)
		INSERT INTO network_optimizer.capacity_search_policies (
			capacity_id, owner_tenant_id, search_mode, target_location_id, forward_search_km, corridor_deviation_km,
			max_deadhead_km, preferred_deadhead_km, max_deadhead_minutes, min_loaded_distance_km,
			max_route_increase_km, objective_profile, allow_unknown_road_distance, version, updated_at
		)
		SELECT owned.id, owned.owner_tenant_id, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1, now()
		FROM owned
		ON CONFLICT (capacity_id) DO UPDATE SET
			search_mode=EXCLUDED.search_mode,
			target_location_id=EXCLUDED.target_location_id, forward_search_km=EXCLUDED.forward_search_km,
			corridor_deviation_km=EXCLUDED.corridor_deviation_km, max_deadhead_km=EXCLUDED.max_deadhead_km,
			preferred_deadhead_km=EXCLUDED.preferred_deadhead_km, max_deadhead_minutes=EXCLUDED.max_deadhead_minutes,
			min_loaded_distance_km=EXCLUDED.min_loaded_distance_km, max_route_increase_km=EXCLUDED.max_route_increase_km,
			objective_profile=EXCLUDED.objective_profile, allow_unknown_road_distance=EXCLUDED.allow_unknown_road_distance,
			version=capacity_search_policies.version+1, updated_at=now()
		WHERE capacity_search_policies.owner_tenant_id = EXCLUDED.owner_tenant_id`,
		capacityID, tenant, nullString(policy.SearchMode), policy.TargetLocationID, policy.ForwardSearchKm, policy.CorridorDeviationKm,
		policy.MaxDeadheadKm, policy.PreferredDeadheadKm, policy.MaxDeadheadMinutes, policy.MinLoadedDistanceKm,
		policy.MaxRouteIncreaseKm, nullString(policy.ObjectiveProfile), policy.AllowUnknownRoadDistance,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) GetCapacityPolicy(ctx context.Context, tenant, capacityID uuid.UUID) (domain.NextLoadSearchPolicy, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT search_mode, target_location_id, forward_search_km, corridor_deviation_km,
			max_deadhead_km, preferred_deadhead_km, max_deadhead_minutes, min_loaded_distance_km,
			max_route_increase_km, objective_profile, allow_unknown_road_distance
		FROM network_optimizer.capacity_search_policies WHERE capacity_id=$1 AND owner_tenant_id=$2`, capacityID, tenant)
	return scanPolicy(row)
}

func scanPolicy(row pgx.Row) (domain.NextLoadSearchPolicy, error) {
	var policy domain.NextLoadSearchPolicy
	var mode, objective *string
	err := row.Scan(&mode, &policy.TargetLocationID, &policy.ForwardSearchKm, &policy.CorridorDeviationKm,
		&policy.MaxDeadheadKm, &policy.PreferredDeadheadKm, &policy.MaxDeadheadMinutes, &policy.MinLoadedDistanceKm,
		&policy.MaxRouteIncreaseKm, &objective, &policy.AllowUnknownRoadDistance)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NextLoadSearchPolicy{}, ErrNotFound
	}
	if err != nil {
		return domain.NextLoadSearchPolicy{}, err
	}
	policy.SearchMode = stringValue(mode)
	policy.ObjectiveProfile = stringValue(objective)
	return policy, nil
}
