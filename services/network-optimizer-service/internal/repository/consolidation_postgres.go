package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) SaveConsolidation(ctx context.Context, run ConsolidationRun, candidates []ConsolidationCandidate) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO network_optimizer.consolidation_search_runs (
			id, tenant_id, capacity_id, capacity_version, pattern,
			started_at, completed_at, status, candidate_limit,
			pool_load_count, evaluated_pair_count, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		run.ID, run.TenantID, run.CapacityID, run.CapacityVersion, run.Pattern,
		run.StartedAt, run.CompletedAt, run.Status, run.CandidateLimit,
		run.PoolLoadCount, run.EvaluatedPairCount, run.CreatedAt,
	); err != nil {
		return err
	}
	for _, candidate := range candidates {
		if _, err := tx.Exec(ctx, `
			INSERT INTO network_optimizer.consolidation_candidates (
				id, search_run_id, tenant_id, capacity_id, pattern, status, execution_supported,
				compatibility_status, compatibility_fingerprint, candidate_fingerprint,
				pickup_overlap_start, pickup_overlap_end, delivery_overlap_start, delivery_overlap_end,
				placement_check, hard_reject_reasons, indeterminate_reason_codes,
				conditions, warnings, capacity_usage, compatibility_trace, created_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,
				$8,$9,$10,
				$11,$12,$13,$14,
				$15,$16,$17,
				$18,$19,$20,$21,$22
			)`,
			candidate.ID, candidate.SearchRunID, candidate.TenantID, candidate.CapacityID, candidate.Pattern, candidate.Status, candidate.ExecutionSupported,
			candidate.CompatibilityStatus, candidate.CompatibilityFingerprint, candidate.CandidateFingerprint,
			candidate.PickupOverlapStart, candidate.PickupOverlapEnd, candidate.DeliveryOverlapStart, candidate.DeliveryOverlapEnd,
			candidate.PlacementCheck, candidate.HardRejectReasons, candidate.IndeterminateReasonCodes,
			jsonOrEmpty(candidate.Conditions, "[]"), jsonOrEmpty(candidate.Warnings, "[]"), nullJSON(candidate.CapacityUsage), jsonOrEmpty(candidate.CompatibilityTrace, "{}"), candidate.CreatedAt,
		); err != nil {
			return err
		}
		for _, member := range candidate.Members {
			if _, err := tx.Exec(ctx, `
				INSERT INTO network_optimizer.consolidation_candidate_members (
					candidate_id, ordinal, load_opportunity_id, load_version, load_owner_tenant_id, created_at
				) VALUES ($1,$2,$3,$4,$5,$6)`,
				candidate.ID, member.Ordinal, member.LoadOpportunityID, member.LoadVersion, member.LoadOwnerTenantID, member.CreatedAt,
			); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) GetConsolidation(ctx context.Context, tenant, id uuid.UUID) (ConsolidationRun, []ConsolidationCandidate, error) {
	var run ConsolidationRun
	err := p.pool.QueryRow(ctx, `
		SELECT id, tenant_id, capacity_id, capacity_version, pattern, started_at, completed_at, status,
		       candidate_limit, pool_load_count, evaluated_pair_count, created_at
		FROM network_optimizer.consolidation_search_runs
		WHERE id = $1 AND tenant_id = $2`, id, tenant).Scan(
		&run.ID, &run.TenantID, &run.CapacityID, &run.CapacityVersion, &run.Pattern, &run.StartedAt, &run.CompletedAt, &run.Status,
		&run.CandidateLimit, &run.PoolLoadCount, &run.EvaluatedPairCount, &run.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConsolidationRun{}, nil, ErrNotFound
	}
	if err != nil {
		return ConsolidationRun{}, nil, err
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, search_run_id, tenant_id, capacity_id, pattern, status, execution_supported,
		       compatibility_status, compatibility_fingerprint, candidate_fingerprint,
		       pickup_overlap_start, pickup_overlap_end, delivery_overlap_start, delivery_overlap_end,
		       placement_check, hard_reject_reasons, indeterminate_reason_codes,
		       conditions, warnings, capacity_usage, compatibility_trace, created_at
		FROM network_optimizer.consolidation_candidates
		WHERE search_run_id = $1 AND tenant_id = $2
		ORDER BY created_at, id`, id, tenant)
	if err != nil {
		return ConsolidationRun{}, nil, err
	}
	defer rows.Close()
	var candidates []ConsolidationCandidate
	for rows.Next() {
		var candidate ConsolidationCandidate
		if err := rows.Scan(
			&candidate.ID, &candidate.SearchRunID, &candidate.TenantID, &candidate.CapacityID, &candidate.Pattern, &candidate.Status, &candidate.ExecutionSupported,
			&candidate.CompatibilityStatus, &candidate.CompatibilityFingerprint, &candidate.CandidateFingerprint,
			&candidate.PickupOverlapStart, &candidate.PickupOverlapEnd, &candidate.DeliveryOverlapStart, &candidate.DeliveryOverlapEnd,
			&candidate.PlacementCheck, &candidate.HardRejectReasons, &candidate.IndeterminateReasonCodes,
			&candidate.Conditions, &candidate.Warnings, &candidate.CapacityUsage, &candidate.CompatibilityTrace, &candidate.CreatedAt,
		); err != nil {
			return ConsolidationRun{}, nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return ConsolidationRun{}, nil, err
	}
	members, err := p.pool.Query(ctx, `
		SELECT m.candidate_id, m.ordinal, m.load_opportunity_id, m.load_version, m.load_owner_tenant_id, m.created_at
		FROM network_optimizer.consolidation_candidate_members m
		JOIN network_optimizer.consolidation_candidates c ON c.id = m.candidate_id
		WHERE c.search_run_id = $1 AND c.tenant_id = $2
		ORDER BY m.candidate_id, m.ordinal`, id, tenant)
	if err != nil {
		return ConsolidationRun{}, nil, err
	}
	defer members.Close()
	byCandidate := map[uuid.UUID][]ConsolidationMember{}
	for members.Next() {
		var candidateID uuid.UUID
		var member ConsolidationMember
		if err := members.Scan(&candidateID, &member.Ordinal, &member.LoadOpportunityID, &member.LoadVersion, &member.LoadOwnerTenantID, &member.CreatedAt); err != nil {
			return ConsolidationRun{}, nil, err
		}
		byCandidate[candidateID] = append(byCandidate[candidateID], member)
	}
	if err := members.Err(); err != nil {
		return ConsolidationRun{}, nil, err
	}
	for i := range candidates {
		candidates[i].Members = byCandidate[candidates[i].ID]
	}
	return run, candidates, nil
}

func jsonOrEmpty(raw []byte, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	return raw
}

func nullJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
