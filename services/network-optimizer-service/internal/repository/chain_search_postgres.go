package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type chainSearchTx struct {
	tx pgx.Tx
}

func (p *Postgres) WithinChainSearch(ctx context.Context, fn func(ChainSearchTx) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&chainSearchTx{tx: tx}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (t *chainSearchTx) InsertChainSearchRun(ctx context.Context, run ChainSearchRun) (ChainSearchRun, error) {
	snapshot, err := marshalServiceDurationSnapshot(run.ServiceDurationSnapshot)
	if err != nil {
		return ChainSearchRun{}, err
	}
	reasonCounts, err := marshalReasonCounts(run.ReasonCounts)
	if err != nil {
		return ChainSearchRun{}, err
	}
	// Claim creation always starts PENDING. Caller-requested statuses are ignored.
	ownerToken := uuid.New()
	if ownerToken == uuid.Nil {
		return ChainSearchRun{}, errors.New("failed to generate owner_token")
	}
	// Claim timing is owned by one database clock seed inside the insert transaction.
	var (
		createdAt      time.Time
		deadlineAt     time.Time
		claimExpiresAt time.Time
	)
	err = t.tx.QueryRow(ctx, `
		WITH claim_clock AS (
			SELECT clock_timestamp() AS ts
		)
		INSERT INTO network_optimizer.chain_search_runs (
			id, tenant_id, status, created_at, started_at, completed_at, cancelled_at,
			request_fingerprint, policy_version, algorithm_version, catalog_fingerprint,
			routing_provider_version, deadline_at, owner_token, claim_expires_at,
			service_duration_policy_id, service_duration_policy_version, service_duration_snapshot_json,
			horizon_start, horizon_end, seed_source,
			seed_capacity_id, seed_capacity_version, seed_shipment_id, seed_shipment_version,
			candidate_count, evaluated_pair_count, provider_call_count,
			search_complete, global_optimum, execution_supported,
			failure_code, cancel_reason_class, reason_counts_json
		)
		SELECT
			$1,$2,$3, claim_clock.ts, NULL, NULL, NULL,
			$4,$5,$6,$7,
			$8, claim_clock.ts + interval '20 seconds', $9, claim_clock.ts + interval '25 seconds',
			$10,$11,$12::jsonb,
			$13,$14,$15,
			$16,$17,$18,$19,
			$20,$21,$22,
			false,false,false,
			NULL,NULL,$23::jsonb
		FROM claim_clock
		RETURNING created_at, deadline_at, claim_expires_at`,
		run.ID, run.TenantID, ChainSearchPending,
		run.RequestFingerprint, run.PolicyVersion, run.AlgorithmVersion, run.CatalogFingerprint,
		run.RoutingProviderVersion, ownerToken,
		run.ServiceDurationPolicyID, run.ServiceDurationPolicyVersion, snapshot,
		run.HorizonStart, run.HorizonEnd, run.SeedSource,
		run.SeedCapacityID, run.SeedCapacityVersion, run.SeedShipmentID, run.SeedShipmentVersion,
		run.CandidateCount, run.EvaluatedPairCount, run.ProviderCallCount,
		reasonCounts,
	).Scan(&createdAt, &deadlineAt, &claimExpiresAt)
	if isUniqueViolation(err) {
		return ChainSearchRun{}, ErrConflict
	}
	if err != nil {
		return ChainSearchRun{}, err
	}
	run.Status = ChainSearchPending
	run.OwnerToken = ownerToken
	run.CreatedAt = createdAt
	run.DeadlineAt = deadlineAt
	run.ClaimExpiresAt = claimExpiresAt
	run.StartedAt = nil
	run.CompletedAt = nil
	run.CancelledAt = nil
	run.SearchComplete = false
	run.FailureCode = nil
	run.CancelReasonClass = nil
	run.GlobalOptimum = false
	run.ExecutionSupported = false
	return run, nil
}

func (t *chainSearchTx) GetChainSearchRun(ctx context.Context, tenant, id uuid.UUID) (ChainSearchRun, error) {
	var (
		run         ChainSearchRun
		snapshotRaw []byte
		reasonRaw   []byte
	)
	err := t.tx.QueryRow(ctx, `
		SELECT id, tenant_id, status, created_at, started_at, completed_at, cancelled_at,
			request_fingerprint, policy_version, algorithm_version, catalog_fingerprint,
			routing_provider_version, deadline_at, owner_token, claim_expires_at,
			service_duration_policy_id, service_duration_policy_version, service_duration_snapshot_json,
			horizon_start, horizon_end, seed_source,
			seed_capacity_id, seed_capacity_version, seed_shipment_id, seed_shipment_version,
			candidate_count, evaluated_pair_count, provider_call_count,
			search_complete, global_optimum, execution_supported,
			failure_code, cancel_reason_class, reason_counts_json
		FROM network_optimizer.chain_search_runs
		WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(
		&run.ID, &run.TenantID, &run.Status, &run.CreatedAt, &run.StartedAt, &run.CompletedAt, &run.CancelledAt,
		&run.RequestFingerprint, &run.PolicyVersion, &run.AlgorithmVersion, &run.CatalogFingerprint,
		&run.RoutingProviderVersion, &run.DeadlineAt, &run.OwnerToken, &run.ClaimExpiresAt,
		&run.ServiceDurationPolicyID, &run.ServiceDurationPolicyVersion, &snapshotRaw,
		&run.HorizonStart, &run.HorizonEnd, &run.SeedSource,
		&run.SeedCapacityID, &run.SeedCapacityVersion, &run.SeedShipmentID, &run.SeedShipmentVersion,
		&run.CandidateCount, &run.EvaluatedPairCount, &run.ProviderCallCount,
		&run.SearchComplete, &run.GlobalOptimum, &run.ExecutionSupported,
		&run.FailureCode, &run.CancelReasonClass, &reasonRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChainSearchRun{}, ErrNotFound
	}
	if err != nil {
		return ChainSearchRun{}, err
	}
	snapshot, err := unmarshalServiceDurationSnapshot(snapshotRaw)
	if err != nil {
		return ChainSearchRun{}, err
	}
	run.ServiceDurationSnapshot = snapshot
	counts, err := unmarshalReasonCounts(reasonRaw)
	if err != nil {
		return ChainSearchRun{}, err
	}
	run.ReasonCounts = counts
	return run, nil
}

func (t *chainSearchTx) LockChainSearchRun(ctx context.Context, tenant, id uuid.UUID) error {
	var found uuid.UUID
	err := t.tx.QueryRow(ctx, `
		SELECT id FROM network_optimizer.chain_search_runs
		WHERE tenant_id=$1 AND id=$2
		FOR UPDATE`, tenant, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (t *chainSearchTx) MarkChainSearchRunning(ctx context.Context, tenant, searchID, ownerToken uuid.UUID) (bool, error) {
	if ownerToken == uuid.Nil {
		return false, errors.New("owner_token required")
	}
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.chain_search_runs
		SET status = $4,
		    started_at = COALESCE(started_at, clock_timestamp())
		WHERE tenant_id = $1
		  AND id = $2
		  AND owner_token = $3
		  AND status = $5
		  AND clock_timestamp() < deadline_at`,
		tenant, searchID, ownerToken, ChainSearchRunning, ChainSearchPending,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func allowedOwnerPersistTransition(from, to string) bool {
	switch from {
	case ChainSearchPending:
		return to == ChainSearchCancelled || to == ChainSearchFailed
	case ChainSearchRunning:
		return to == ChainSearchCompleted || to == ChainSearchCancelled || to == ChainSearchFailed
	default:
		return false
	}
}

func (t *chainSearchTx) OwnerPersistChainSearchRun(ctx context.Context, run ChainSearchRun, ownerToken uuid.UUID, expectedStatus string) (bool, error) {
	if ownerToken == uuid.Nil {
		return false, errors.New("owner_token required")
	}
	if expectedStatus != ChainSearchPending && expectedStatus != ChainSearchRunning {
		return false, errors.New("expectedStatus must be nonterminal")
	}
	target := run.Status
	if target == ChainSearchRunning && expectedStatus == ChainSearchPending {
		return false, errors.New("PENDING to RUNNING requires MarkChainSearchRunning")
	}
	if !allowedOwnerPersistTransition(expectedStatus, target) {
		return false, errors.New("owner persist transition not allowed")
	}
	reasonCounts, err := marshalReasonCounts(run.ReasonCounts)
	if err != nil {
		return false, err
	}
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.chain_search_runs SET
			status = $3,
			started_at = COALESCE(started_at, $4),
			completed_at = $5,
			cancelled_at = $6,
			candidate_count = $7,
			evaluated_pair_count = $8,
			provider_call_count = $9,
			search_complete = $10,
			failure_code = $11,
			cancel_reason_class = $12,
			reason_counts_json = $13::jsonb
		WHERE tenant_id = $1
		  AND id = $2
		  AND owner_token = $14
		  AND status = $15
		  AND status IN ('PENDING', 'RUNNING')
		  AND clock_timestamp() < claim_expires_at`,
		run.TenantID, run.ID,
		run.Status, run.StartedAt, run.CompletedAt, run.CancelledAt,
		run.CandidateCount, run.EvaluatedPairCount, run.ProviderCallCount,
		run.SearchComplete, run.FailureCode, run.CancelReasonClass, reasonCounts,
		ownerToken, expectedStatus,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (t *chainSearchTx) RecoverExpiredChainSearchClaim(ctx context.Context, tenant, searchID uuid.UUID) (bool, error) {
	failure := ChainInterruptedFailureCode
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.chain_search_runs
		SET status = $3,
		    search_complete = false,
		    failure_code = $4,
		    completed_at = COALESCE(completed_at, clock_timestamp()),
		    cancel_reason_class = NULL
		WHERE tenant_id = $1
		  AND id = $2
		  AND status IN ('PENDING', 'RUNNING')
		  AND claim_expires_at <= clock_timestamp()`,
		tenant, searchID, ChainSearchFailed, failure,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (t *chainSearchTx) InsertChainCandidate(ctx context.Context, candidate ChainCandidate) error {
	omitted := candidate.OmittedComponents
	if omitted == nil {
		omitted = []string{}
	}
	reasons := candidate.ReasonCodes
	if reasons == nil {
		reasons = []string{}
	}
	_, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.chain_candidates (
			candidate_id, search_id, tenant_id, ordinal,
			first_load_id, first_load_version, second_load_id, second_load_version,
			feasibility_status, selected_order,
			empty_road_distance_m, total_duration_seconds, utilization, target_distance_m,
			revenue_amount, revenue_currency, omitted_components, reason_codes,
			route_plan_id, candidate_fingerprint, created_at
		) VALUES (
			$1,$2,$3,$4,
			$5,$6,$7,$8,
			$9,$10,
			$11,$12,$13,$14,
			$15,$16,$17,$18,
			$19,$20,$21
		)`,
		candidate.CandidateID, candidate.SearchID, candidate.TenantID, candidate.Ordinal,
		candidate.FirstLoadID, candidate.FirstLoadVersion, candidate.SecondLoadID, candidate.SecondLoadVersion,
		candidate.FeasibilityStatus, candidate.SelectedOrder,
		candidate.EmptyRoadDistanceM, candidate.TotalDurationSeconds, candidate.Utilization, candidate.TargetDistanceM,
		candidate.RevenueAmount, candidate.RevenueCurrency, omitted, reasons,
		candidate.RoutePlanID, candidate.CandidateFingerprint, candidate.CreatedAt,
	)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (t *chainSearchTx) ListChainCandidates(ctx context.Context, tenant, searchID uuid.UUID) ([]ChainCandidate, error) {
	rows, err := t.tx.Query(ctx, `
		SELECT candidate_id, search_id, tenant_id, ordinal,
			first_load_id, first_load_version, second_load_id, second_load_version,
			feasibility_status, selected_order,
			empty_road_distance_m, total_duration_seconds, utilization, target_distance_m,
			revenue_amount, revenue_currency, omitted_components, reason_codes,
			route_plan_id, candidate_fingerprint, created_at
		FROM network_optimizer.chain_candidates
		WHERE tenant_id=$1 AND search_id=$2
		ORDER BY ordinal`, tenant, searchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChainCandidate
	for rows.Next() {
		var candidate ChainCandidate
		if err := rows.Scan(
			&candidate.CandidateID, &candidate.SearchID, &candidate.TenantID, &candidate.Ordinal,
			&candidate.FirstLoadID, &candidate.FirstLoadVersion, &candidate.SecondLoadID, &candidate.SecondLoadVersion,
			&candidate.FeasibilityStatus, &candidate.SelectedOrder,
			&candidate.EmptyRoadDistanceM, &candidate.TotalDurationSeconds, &candidate.Utilization, &candidate.TargetDistanceM,
			&candidate.RevenueAmount, &candidate.RevenueCurrency, &candidate.OmittedComponents, &candidate.ReasonCodes,
			&candidate.RoutePlanID, &candidate.CandidateFingerprint, &candidate.CreatedAt,
		); err != nil {
			return nil, err
		}
		if candidate.OmittedComponents == nil {
			candidate.OmittedComponents = []string{}
		}
		if candidate.ReasonCodes == nil {
			candidate.ReasonCodes = []string{}
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (t *chainSearchTx) InsertChainSearchIdempotency(ctx context.Context, row ChainSearchIdempotency) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.chain_search_idempotency (
			tenant_id, idempotency_key, request_fingerprint, search_id, created_at
		) VALUES ($1,$2,$3,$4,$5)`,
		row.TenantID, row.IdempotencyKey, row.RequestFingerprint, row.SearchID, row.CreatedAt,
	)
	if isUniqueViolation(err) {
		return ErrIdempotencyRace
	}
	return err
}

func (t *chainSearchTx) GetChainSearchIdempotency(ctx context.Context, tenant uuid.UUID, key string) (ChainSearchIdempotency, error) {
	var row ChainSearchIdempotency
	err := t.tx.QueryRow(ctx, `
		SELECT tenant_id, idempotency_key, request_fingerprint, search_id, created_at
		FROM network_optimizer.chain_search_idempotency
		WHERE tenant_id=$1 AND idempotency_key=$2`, tenant, key).Scan(
		&row.TenantID, &row.IdempotencyKey, &row.RequestFingerprint, &row.SearchID, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChainSearchIdempotency{}, ErrNotFound
	}
	return row, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Ensure Postgres satisfies ChainSearchStore without forcing callers through the general Store Tx.
var _ ChainSearchStore = (*Postgres)(nil)

// DeadlineAndClaimExpiry returns the frozen 20s compute deadline and +5s claim grace.
func DeadlineAndClaimExpiry(createdAt time.Time) (deadlineAt, claimExpiresAt time.Time) {
	deadlineAt = createdAt.UTC().Add(20 * time.Second)
	claimExpiresAt = deadlineAt.Add(5 * time.Second)
	return deadlineAt, claimExpiresAt
}
