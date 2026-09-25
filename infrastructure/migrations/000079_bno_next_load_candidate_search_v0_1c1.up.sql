-- BNO-0.1C1 next-load candidate search.
-- Location master remains transport.locations.

ALTER TABLE network_optimizer.carrier_search_policies
    ADD COLUMN radius_km double precision,
    ADD CONSTRAINT carrier_search_radius_chk CHECK (radius_km IS NULL OR radius_km > 0);

ALTER TABLE network_optimizer.capacity_search_policies
    ADD COLUMN radius_km double precision,
    ADD CONSTRAINT capacity_search_radius_chk CHECK (radius_km IS NULL OR radius_km > 0);

CREATE TABLE network_optimizer.next_load_search_runs (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    capacity_id uuid NOT NULL,
    capacity_version integer NOT NULL,
    effective_policy_fingerprint text NOT NULL,
    started_at timestamptz NOT NULL,
    completed_at timestamptz NOT NULL,
    routing_provider text NOT NULL,
    status text NOT NULL,
    CONSTRAINT next_load_search_runs_id_tenant_uidx UNIQUE (id, tenant_id),
    CONSTRAINT next_load_search_runs_id_tenant_capacity_uidx UNIQUE (id, tenant_id, capacity_id),
    CONSTRAINT next_load_search_runs_capacity_owner_fk FOREIGN KEY (capacity_id, tenant_id)
        REFERENCES network_optimizer.capacities (id, owner_tenant_id)
);

CREATE INDEX next_load_search_runs_tenant_capacity_idx
    ON network_optimizer.next_load_search_runs (tenant_id, capacity_id);

CREATE TABLE network_optimizer.match_candidates (
    id uuid PRIMARY KEY,
    search_run_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    capacity_id uuid NOT NULL,
    load_opportunity_id uuid NOT NULL,
    load_version integer NOT NULL,
    eligibility text NOT NULL,
    reject_reasons text[] NOT NULL DEFAULT '{}',
    road_deadhead_km double precision,
    road_deadhead_minutes double precision,
    waiting_minutes double precision,
    compatibility_status text NOT NULL,
    compatibility_fingerprint text NOT NULL,
    policy_fingerprint text NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT match_candidates_eligibility_chk CHECK (eligibility IN ('ELIGIBLE', 'REJECTED')),
    CONSTRAINT match_candidates_id_tenant_uidx UNIQUE (id, tenant_id),
    CONSTRAINT match_candidates_run_capacity_fk FOREIGN KEY (search_run_id, tenant_id, capacity_id)
        REFERENCES network_optimizer.next_load_search_runs (id, tenant_id, capacity_id)
);

CREATE INDEX match_candidates_run_idx
    ON network_optimizer.match_candidates (search_run_id, tenant_id);
