-- NLO-0.3B pairwise same-origin / same-destination consolidation.
-- Opt-in flags default false. Existing marketplace indexes are unchanged.

ALTER TABLE network_optimizer.load_opportunities
    ADD COLUMN consolidation_allowed boolean NOT NULL DEFAULT false,
    ADD COLUMN cross_shipper_consolidation_allowed boolean NOT NULL DEFAULT false;

CREATE INDEX load_opportunities_published_opt_in_od_idx
    ON network_optimizer.load_opportunities (pickup_location_id, delivery_location_id, id)
    WHERE status = 'PUBLISHED'
      AND (consolidation_allowed OR cross_shipper_consolidation_allowed);

CREATE TABLE network_optimizer.consolidation_search_runs (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    capacity_id uuid NOT NULL,
    capacity_version integer NOT NULL,
    pattern text NOT NULL,
    started_at timestamptz NOT NULL,
    completed_at timestamptz NOT NULL,
    status text NOT NULL,
    candidate_limit integer,
    pool_load_count integer NOT NULL,
    evaluated_pair_count integer NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT consolidation_search_runs_pattern_chk CHECK (pattern = 'SAME_ORIGIN_SAME_DESTINATION'),
    CONSTRAINT consolidation_search_runs_status_chk CHECK (status = 'COMPLETED'),
    CONSTRAINT consolidation_search_runs_limit_chk CHECK (candidate_limit IS NULL OR candidate_limit >= 0),
    CONSTRAINT consolidation_search_runs_id_tenant_uidx UNIQUE (id, tenant_id),
    CONSTRAINT consolidation_search_runs_capacity_owner_fk FOREIGN KEY (capacity_id, tenant_id)
        REFERENCES network_optimizer.capacities (id, owner_tenant_id)
);

CREATE INDEX consolidation_search_runs_tenant_capacity_idx
    ON network_optimizer.consolidation_search_runs (tenant_id, capacity_id);

CREATE TABLE network_optimizer.consolidation_candidates (
    id uuid PRIMARY KEY,
    search_run_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    capacity_id uuid NOT NULL,
    pattern text NOT NULL,
    status text NOT NULL,
    execution_supported boolean NOT NULL,
    compatibility_status text NOT NULL,
    compatibility_fingerprint text NOT NULL,
    candidate_fingerprint text NOT NULL,
    pickup_overlap_start timestamptz,
    pickup_overlap_end timestamptz,
    delivery_overlap_start timestamptz,
    delivery_overlap_end timestamptz,
    placement_check text NOT NULL,
    hard_reject_reasons text[] NOT NULL DEFAULT '{}',
    indeterminate_reason_codes text[] NOT NULL DEFAULT '{}',
    conditions jsonb NOT NULL DEFAULT '[]',
    warnings jsonb NOT NULL DEFAULT '[]',
    capacity_usage jsonb,
    compatibility_trace jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    CONSTRAINT consolidation_candidates_status_chk CHECK (status IN ('HARD_REJECT', 'INDETERMINATE', 'FEASIBLE', 'PROPOSED', 'INVALIDATED')),
    CONSTRAINT consolidation_candidates_execution_chk CHECK (execution_supported = false),
    CONSTRAINT consolidation_candidates_placement_chk CHECK (placement_check = 'NOT_EVALUATED'),
    CONSTRAINT consolidation_candidates_run_fk FOREIGN KEY (search_run_id, tenant_id)
        REFERENCES network_optimizer.consolidation_search_runs (id, tenant_id),
    CONSTRAINT consolidation_candidates_fingerprint_uidx UNIQUE (search_run_id, candidate_fingerprint)
);

CREATE INDEX consolidation_candidates_run_idx
    ON network_optimizer.consolidation_candidates (search_run_id, tenant_id);

CREATE TABLE network_optimizer.consolidation_candidate_members (
    candidate_id uuid NOT NULL,
    ordinal integer NOT NULL,
    load_opportunity_id uuid NOT NULL,
    load_version integer NOT NULL,
    load_owner_tenant_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (candidate_id, ordinal),
    CONSTRAINT consolidation_candidate_members_load_uidx UNIQUE (candidate_id, load_opportunity_id),
    CONSTRAINT consolidation_candidate_members_ordinal_chk CHECK (ordinal IN (1, 2)),
    CONSTRAINT consolidation_candidate_members_candidate_fk FOREIGN KEY (candidate_id)
        REFERENCES network_optimizer.consolidation_candidates (id)
);
