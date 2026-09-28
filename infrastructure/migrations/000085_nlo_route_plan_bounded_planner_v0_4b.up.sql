-- NLO-0.4B persistent route plan. Planning rows only. No route_plan_activations.

CREATE TABLE network_optimizer.route_plans (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    version integer NOT NULL CHECK (version >= 1),
    status text NOT NULL CHECK (status IN ('EVALUATED', 'ACCEPTED', 'SUPERSEDED', 'CANCELLED')),
    planning_mode text NOT NULL CHECK (planning_mode IN ('CURRENT_TRIP', 'DEPOT_START')),
    result_status text NOT NULL CHECK (result_status IN ('FEASIBLE_PLAN_FOUND', 'INDETERMINATE_PLAN_FOUND')),
    capacity_id uuid NULL,
    capacity_version integer NULL,
    shipment_id uuid NULL,
    shipment_version integer NULL,
    vehicle_id uuid NULL,
    context_fingerprint text NOT NULL,
    evaluation_fingerprint text NOT NULL,
    algorithm_policy_version text NOT NULL,
    routing_policy_version text NOT NULL,
    supersedes_plan_id uuid NULL,
    execution_supported boolean NOT NULL CHECK (execution_supported = false),
    reason_codes text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    accepted_at timestamptz NULL,
    cancelled_at timestamptz NULL,
    superseded_at timestamptz NULL,
    CONSTRAINT route_plans_current_trip_chk CHECK (
        planning_mode <> 'CURRENT_TRIP'
        OR (
            shipment_id IS NOT NULL
            AND shipment_version >= 1
            AND capacity_id IS NULL
            AND capacity_version IS NULL
        )
    ),
    CONSTRAINT route_plans_depot_start_chk CHECK (
        planning_mode <> 'DEPOT_START'
        OR (
            capacity_id IS NOT NULL
            AND capacity_version >= 1
            AND shipment_id IS NULL
            AND shipment_version IS NULL
        )
    )
);

CREATE INDEX route_plans_tenant_created_idx
    ON network_optimizer.route_plans (tenant_id, created_at DESC, id);

CREATE TABLE network_optimizer.route_plan_stops (
    id uuid PRIMARY KEY,
    route_plan_id uuid NOT NULL REFERENCES network_optimizer.route_plans (id),
    ordinal integer NOT NULL CHECK (ordinal BETWEEN 1 AND 8),
    stop_role text NOT NULL CHECK (stop_role IN ('START', 'CARGO', 'END')),
    point_kind text NOT NULL CHECK (point_kind IN ('CANONICAL_LOCATION', 'POSITION_ANCHOR')),
    location_id uuid NULL,
    latitude double precision NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude double precision NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    point_source text NOT NULL CHECK (point_source IN ('CANONICAL_LOCATION', 'TRACKING_POSITION', 'CAPACITY_POSITION')),
    point_observed_at timestamptz NULL,
    planned_arrival timestamptz NULL,
    planned_departure timestamptz NULL,
    service_duration_seconds integer NULL CHECK (service_duration_seconds IS NULL OR service_duration_seconds >= 0),
    UNIQUE (route_plan_id, ordinal),
    CONSTRAINT route_plan_stops_canonical_chk CHECK (
        point_kind <> 'CANONICAL_LOCATION'
        OR (location_id IS NOT NULL AND point_source = 'CANONICAL_LOCATION')
    ),
    CONSTRAINT route_plan_stops_anchor_chk CHECK (
        point_kind <> 'POSITION_ANCHOR'
        OR (location_id IS NULL AND stop_role = 'START' AND point_source IN ('TRACKING_POSITION', 'CAPACITY_POSITION'))
    ),
    CONSTRAINT route_plan_stops_role_point_chk CHECK (
        (stop_role = 'CARGO' AND point_kind = 'CANONICAL_LOCATION')
        OR (stop_role = 'END' AND point_kind = 'CANONICAL_LOCATION')
        OR (stop_role = 'START' AND point_kind IN ('CANONICAL_LOCATION', 'POSITION_ANCHOR'))
    )
);

CREATE UNIQUE INDEX route_plan_stops_one_start_idx
    ON network_optimizer.route_plan_stops (route_plan_id)
    WHERE stop_role = 'START';

CREATE UNIQUE INDEX route_plan_stops_one_end_idx
    ON network_optimizer.route_plan_stops (route_plan_id)
    WHERE stop_role = 'END';

ALTER TABLE network_optimizer.route_plan_stops
    ADD CONSTRAINT route_plan_stops_plan_id_key UNIQUE (route_plan_id, id);

CREATE TABLE network_optimizer.route_stop_actions (
    id uuid PRIMARY KEY,
    route_plan_id uuid NOT NULL REFERENCES network_optimizer.route_plans (id),
    stop_id uuid NOT NULL REFERENCES network_optimizer.route_plan_stops (id),
    action_ordinal integer NOT NULL CHECK (action_ordinal BETWEEN 1 AND 4),
    action_type text NOT NULL CHECK (action_type IN ('PICKUP', 'DELIVERY')),
    subject_type text NOT NULL CHECK (subject_type IN ('LOAD_OPPORTUNITY', 'SHIPMENT_CARGO')),
    subject_id uuid NOT NULL,
    subject_version integer NOT NULL CHECK (subject_version >= 1),
    weight_delta_kg double precision NULL,
    volume_delta_m3 double precision NULL,
    pallet_delta double precision NULL,
    linear_meters_delta double precision NULL,
    window_start timestamptz NULL,
    window_end timestamptz NULL,
    source_shipment_id uuid NULL,
    source_shipment_version integer NULL,
    evidence_state text NULL,
    evidence_state_version integer NULL,
    evidence_occurred_at timestamptz NULL,
    public_subject_snapshot jsonb NULL,
    UNIQUE (stop_id, action_ordinal),
    CONSTRAINT route_stop_actions_same_plan_fk FOREIGN KEY (route_plan_id, stop_id)
        REFERENCES network_optimizer.route_plan_stops (route_plan_id, id),
    CONSTRAINT route_stop_actions_onboard_provenance_chk CHECK (
        subject_type <> 'SHIPMENT_CARGO'
        OR (
            action_type = 'DELIVERY'
            AND source_shipment_id IS NOT NULL
            AND source_shipment_version >= 1
            AND evidence_state IS NOT NULL
            AND evidence_state_version IS NOT NULL
            AND evidence_occurred_at IS NOT NULL
        )
    )
);

CREATE TABLE network_optimizer.route_plan_legs (
    id uuid PRIMARY KEY,
    route_plan_id uuid NOT NULL REFERENCES network_optimizer.route_plans (id),
    ordinal integer NOT NULL CHECK (ordinal >= 1),
    from_stop_id uuid NOT NULL REFERENCES network_optimizer.route_plan_stops (id),
    to_stop_id uuid NOT NULL REFERENCES network_optimizer.route_plan_stops (id),
    from_point_fingerprint text NOT NULL,
    to_point_fingerprint text NOT NULL,
    distance_m integer NOT NULL CHECK (distance_m >= 0),
    duration_seconds integer NOT NULL CHECK (duration_seconds >= 0),
    provider text NOT NULL,
    request_fingerprint text NOT NULL,
    response_fingerprint text NOT NULL,
    vehicle_profile_hash text NOT NULL,
    route_mode text NOT NULL,
    traffic_mode text NOT NULL,
    departure_bucket text NOT NULL DEFAULT '',
    calculated_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    provider_default_used boolean NOT NULL,
    UNIQUE (route_plan_id, ordinal),
    CONSTRAINT route_plan_legs_from_same_plan_fk FOREIGN KEY (route_plan_id, from_stop_id)
        REFERENCES network_optimizer.route_plan_stops (route_plan_id, id),
    CONSTRAINT route_plan_legs_to_same_plan_fk FOREIGN KEY (route_plan_id, to_stop_id)
        REFERENCES network_optimizer.route_plan_stops (route_plan_id, id)
);

CREATE TABLE network_optimizer.route_capacity_snapshots (
    id uuid PRIMARY KEY,
    route_plan_id uuid NOT NULL REFERENCES network_optimizer.route_plans (id),
    sequence_ordinal integer NOT NULL CHECK (sequence_ordinal >= 0),
    after_stop_id uuid NOT NULL REFERENCES network_optimizer.route_plan_stops (id),
    after_action_ordinal integer NOT NULL CHECK (after_action_ordinal >= 0),
    payload_status text NOT NULL CHECK (payload_status IN ('KNOWN', 'UNKNOWN')),
    payload_remaining_kg double precision NULL,
    volume_status text NOT NULL CHECK (volume_status IN ('KNOWN', 'UNKNOWN')),
    volume_remaining_m3 double precision NULL,
    pallet_status text NOT NULL CHECK (pallet_status IN ('KNOWN', 'UNKNOWN')),
    pallet_positions_remaining double precision NULL,
    linear_status text NOT NULL CHECK (linear_status IN ('KNOWN', 'UNKNOWN')),
    linear_meters_remaining double precision NULL,
    height_status text NOT NULL CHECK (height_status IN ('KNOWN', 'UNKNOWN')),
    height_remaining_mm double precision NULL,
    temperature_allocation_status text NOT NULL,
    compatibility_status text NULL,
    compatibility_fingerprint text NULL,
    temperature_check_status text NULL,
    adr_check_status text NULL,
    food_grade_check_status text NULL,
    UNIQUE (route_plan_id, sequence_ordinal),
    CONSTRAINT route_capacity_snapshots_same_plan_fk FOREIGN KEY (route_plan_id, after_stop_id)
        REFERENCES network_optimizer.route_plan_stops (route_plan_id, id),
    CONSTRAINT route_capacity_snapshots_unknown_not_zero_chk CHECK (
        (payload_status = 'UNKNOWN') = (payload_remaining_kg IS NULL)
        AND (volume_status = 'UNKNOWN') = (volume_remaining_m3 IS NULL)
        AND (pallet_status = 'UNKNOWN') = (pallet_positions_remaining IS NULL)
        AND (linear_status = 'UNKNOWN') = (linear_meters_remaining IS NULL)
        AND (height_status = 'UNKNOWN') = (height_remaining_mm IS NULL)
    )
);

CREATE TABLE network_optimizer.route_plan_dependencies (
    id uuid PRIMARY KEY,
    route_plan_id uuid NOT NULL REFERENCES network_optimizer.route_plans (id),
    dependency_kind text NOT NULL CHECK (dependency_kind IN (
        'SHIPMENT', 'CAPACITY', 'VEHICLE', 'LOAD_OPPORTUNITY', 'SHIPMENT_CARGO',
        'CURRENT_TRIP_CONTEXT', 'ROUTING_POLICY', 'CATALOG', 'RULE_SET', 'ALGORITHM_POLICY'
    )),
    subject_id uuid NULL,
    subject_version integer NULL,
    fingerprint text NULL,
    UNIQUE (route_plan_id, dependency_kind, subject_id)
);
