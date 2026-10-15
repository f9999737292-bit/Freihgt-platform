-- NLO-0.9C-I2A chain-search persistence foundation.
-- Owned by network-optimizer. No public API activation. execution_supported stays false.
-- Does not accept/activate route plans or hand off to TMS.

-- Supporting unique key for tenant-safe composite FKs from chain candidates.
ALTER TABLE network_optimizer.route_plans
    ADD CONSTRAINT route_plans_tenant_id_uidx UNIQUE (tenant_id, id);

CREATE TABLE network_optimizer.chain_search_runs (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    status text NOT NULL,
    created_at timestamptz NOT NULL,
    started_at timestamptz NULL,
    completed_at timestamptz NULL,
    cancelled_at timestamptz NULL,
    request_fingerprint text NOT NULL,
    policy_version text NOT NULL,
    algorithm_version text NOT NULL,
    catalog_fingerprint text NOT NULL,
    routing_provider_version text NOT NULL,
    deadline_at timestamptz NOT NULL,
    owner_token uuid NOT NULL,
    claim_expires_at timestamptz NOT NULL,
    service_duration_policy_id uuid NULL,
    service_duration_policy_version integer NULL,
    service_duration_snapshot_json jsonb NOT NULL,
    horizon_start timestamptz NOT NULL,
    horizon_end timestamptz NOT NULL,
    seed_source text NOT NULL,
    seed_capacity_id uuid NULL,
    seed_capacity_version integer NULL,
    seed_shipment_id uuid NULL,
    seed_shipment_version integer NULL,
    candidate_count integer NOT NULL,
    evaluated_pair_count integer NOT NULL,
    provider_call_count integer NOT NULL,
    search_complete boolean NOT NULL,
    global_optimum boolean NOT NULL,
    execution_supported boolean NOT NULL,
    failure_code text NULL,
    cancel_reason_class text NULL,
    reason_counts_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT chain_search_runs_tenant_id_uidx UNIQUE (tenant_id, id),
    CONSTRAINT chain_search_runs_tenant_id_fp_uidx UNIQUE (tenant_id, id, request_fingerprint),
    CONSTRAINT chain_search_runs_status_chk CHECK (
        status IN ('PENDING', 'RUNNING', 'COMPLETED', 'CANCELLED', 'FAILED')
    ),
    CONSTRAINT chain_search_runs_fingerprint_chk CHECK (request_fingerprint <> ''),
    CONSTRAINT chain_search_runs_policy_chk CHECK (policy_version <> ''),
    CONSTRAINT chain_search_runs_algorithm_chk CHECK (algorithm_version <> ''),
    CONSTRAINT chain_search_runs_catalog_chk CHECK (catalog_fingerprint <> ''),
    CONSTRAINT chain_search_runs_provider_chk CHECK (routing_provider_version <> ''),
    CONSTRAINT chain_search_runs_global_optimum_chk CHECK (global_optimum = false),
    CONSTRAINT chain_search_runs_execution_chk CHECK (execution_supported = false),
    CONSTRAINT chain_search_runs_counts_chk CHECK (
        candidate_count >= 0
        AND candidate_count <= 4
        AND evaluated_pair_count >= 0
        AND provider_call_count >= 0
    ),
    CONSTRAINT chain_search_runs_seed_source_chk CHECK (
        seed_source IN ('MANUAL_AVAILABLE_CAPACITY', 'CURRENT_TRIP_RESIDUAL')
    ),
    CONSTRAINT chain_search_runs_seed_manual_chk CHECK (
        seed_source <> 'MANUAL_AVAILABLE_CAPACITY'
        OR (
            seed_capacity_id IS NOT NULL
            AND seed_capacity_version IS NOT NULL
            AND seed_capacity_version >= 1
            AND seed_shipment_id IS NULL
            AND seed_shipment_version IS NULL
        )
    ),
    CONSTRAINT chain_search_runs_seed_trip_chk CHECK (
        seed_source <> 'CURRENT_TRIP_RESIDUAL'
        OR (
            seed_shipment_id IS NOT NULL
            AND seed_shipment_version IS NOT NULL
            AND seed_shipment_version >= 1
            AND seed_capacity_id IS NULL
            AND seed_capacity_version IS NULL
        )
    ),
    CONSTRAINT chain_search_runs_claim_window_chk CHECK (
        deadline_at > created_at
        AND claim_expires_at > deadline_at
    ),
    CONSTRAINT chain_search_runs_search_complete_chk CHECK (
        (status = 'COMPLETED' AND search_complete = true)
        OR (status <> 'COMPLETED' AND search_complete = false)
    ),
    CONSTRAINT chain_search_runs_duration_version_chk CHECK (
        service_duration_policy_version IS NULL
        OR service_duration_policy_version >= 1
    )
);

CREATE INDEX chain_search_runs_tenant_created_idx
    ON network_optimizer.chain_search_runs (tenant_id, created_at DESC, id);

CREATE INDEX chain_search_runs_tenant_status_idx
    ON network_optimizer.chain_search_runs (tenant_id, status);

CREATE INDEX chain_search_runs_expired_claim_idx
    ON network_optimizer.chain_search_runs (claim_expires_at, id)
    WHERE status IN ('PENDING', 'RUNNING');

CREATE TABLE network_optimizer.chain_candidates (
    candidate_id uuid PRIMARY KEY,
    search_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    ordinal integer NOT NULL,
    first_load_id uuid NOT NULL,
    first_load_version integer NOT NULL,
    second_load_id uuid NOT NULL,
    second_load_version integer NOT NULL,
    feasibility_status text NOT NULL,
    selected_order text NULL,
    empty_road_distance_m double precision NULL,
    total_duration_seconds integer NULL,
    utilization double precision NULL,
    target_distance_m double precision NULL,
    revenue_amount bigint NULL,
    revenue_currency text NULL,
    omitted_components text[] NOT NULL DEFAULT '{}',
    reason_codes text[] NOT NULL DEFAULT '{}',
    route_plan_id uuid NULL,
    candidate_fingerprint text NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT chain_candidates_tenant_search_fk FOREIGN KEY (tenant_id, search_id)
        REFERENCES network_optimizer.chain_search_runs (tenant_id, id)
        ON DELETE CASCADE,
    CONSTRAINT chain_candidates_tenant_route_plan_fk FOREIGN KEY (tenant_id, route_plan_id)
        REFERENCES network_optimizer.route_plans (tenant_id, id)
        MATCH SIMPLE
        ON DELETE RESTRICT,
    CONSTRAINT chain_candidates_pair_uidx UNIQUE (
        search_id, first_load_id, second_load_id, first_load_version, second_load_version
    ),
    CONSTRAINT chain_candidates_ordinal_uidx UNIQUE (search_id, ordinal),
    CONSTRAINT chain_candidates_ordinal_chk CHECK (ordinal BETWEEN 1 AND 4),
    CONSTRAINT chain_candidates_load_versions_chk CHECK (
        first_load_version >= 1 AND second_load_version >= 1
    ),
    CONSTRAINT chain_candidates_canonical_pair_chk CHECK (first_load_id < second_load_id),
    CONSTRAINT chain_candidates_feasibility_chk CHECK (
        feasibility_status IN ('FEASIBLE', 'INFEASIBLE', 'UNFINISHED')
    ),
    CONSTRAINT chain_candidates_selected_order_chk CHECK (
        selected_order IS NULL
        OR selected_order IN ('L1_BEFORE_L2', 'L2_BEFORE_L1')
    ),
    CONSTRAINT chain_candidates_fingerprint_chk CHECK (candidate_fingerprint <> ''),
    CONSTRAINT chain_candidates_revenue_currency_chk CHECK (
        (revenue_amount IS NULL AND revenue_currency IS NULL)
        OR (revenue_amount IS NOT NULL AND revenue_currency IS NOT NULL AND revenue_currency <> '')
    )
);

CREATE INDEX chain_candidates_tenant_search_idx
    ON network_optimizer.chain_candidates (tenant_id, search_id);

CREATE INDEX chain_candidates_search_ordinal_idx
    ON network_optimizer.chain_candidates (search_id, ordinal);

CREATE TABLE network_optimizer.chain_search_idempotency (
    tenant_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    request_fingerprint text NOT NULL,
    search_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, idempotency_key),
    CONSTRAINT chain_search_idempotency_key_chk CHECK (idempotency_key <> ''),
    CONSTRAINT chain_search_idempotency_fingerprint_chk CHECK (request_fingerprint <> ''),
    CONSTRAINT chain_search_idempotency_search_fk FOREIGN KEY (tenant_id, search_id, request_fingerprint)
        REFERENCES network_optimizer.chain_search_runs (tenant_id, id, request_fingerprint)
        ON DELETE RESTRICT
);

CREATE INDEX chain_search_idempotency_search_idx
    ON network_optimizer.chain_search_idempotency (search_id);
