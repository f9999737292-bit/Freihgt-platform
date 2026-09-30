-- TMS-MSTOP-0.1E control tower execution read model.
-- Separate from shipment_status_projection. No shipment_tenant_id. No coordinates.

CREATE SCHEMA IF NOT EXISTS control_tower;

CREATE TABLE control_tower.execution_event_inbox (
    event_id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    revision_id uuid NULL,
    event_sequence bigint NULL,
    event_type text NOT NULL,
    topic text NOT NULL,
    partition_id integer NOT NULL,
    message_offset bigint NOT NULL,
    payload_sha256 text NOT NULL,
    processing_outcome text NOT NULL,
    occurred_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL,
    processed_at timestamptz NOT NULL,
    held_payload jsonb NULL,
    CONSTRAINT uq_execution_event_inbox_position UNIQUE (topic, partition_id, message_offset),
    CONSTRAINT chk_execution_event_inbox_sequence CHECK (event_sequence IS NULL OR event_sequence > 0)
);

CREATE TABLE control_tower.execution_projection (
    execution_id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    active_revision_id uuid NOT NULL,
    carrier_company_id uuid NULL,
    driver_id uuid NULL,
    vehicle_id uuid NULL,
    current_stop_id uuid NULL,
    last_event_sequence bigint NOT NULL CHECK (last_event_sequence >= 0),
    last_event_id uuid NOT NULL,
    last_event_type text NOT NULL,
    last_occurred_at timestamptz NOT NULL,
    last_consumed_at timestamptz NOT NULL,
    gap_detected boolean NOT NULL DEFAULT false,
    gap_from_sequence bigint NULL,
    gap_to_sequence bigint NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT chk_execution_projection_gap CHECK (
        (gap_detected = false AND gap_from_sequence IS NULL AND gap_to_sequence IS NULL)
        OR (gap_detected = true AND gap_from_sequence IS NOT NULL AND gap_to_sequence IS NOT NULL AND gap_from_sequence <= gap_to_sequence)
    )
);

CREATE INDEX idx_execution_projection_operating_tenant
    ON control_tower.execution_projection (operating_tenant_id);

CREATE TABLE control_tower.execution_stop_projection (
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    revision_id uuid NOT NULL,
    execution_stop_id uuid NOT NULL,
    ordinal integer NOT NULL CHECK (ordinal >= 0),
    stop_role text NOT NULL,
    point_kind text NOT NULL,
    location_id uuid NULL,
    planned_arrival timestamptz NULL,
    planned_departure timestamptz NULL,
    status text NOT NULL CHECK (status IN (
        'PLANNED', 'ARRIVED', 'SERVICE_STARTED', 'COMPLETED', 'CANCELLED', 'SKIPPED', 'SUPERSEDED'
    )),
    arrived_at timestamptz NULL,
    service_started_at timestamptz NULL,
    completed_at timestamptz NULL,
    approaching_at timestamptz NULL,
    approach_distance_meters double precision NULL,
    status_reason text NULL,
    last_event_sequence bigint NOT NULL CHECK (last_event_sequence >= 0),
    last_event_id uuid NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (execution_id, revision_id, execution_stop_id)
);

CREATE INDEX idx_execution_stop_projection_tenant
    ON control_tower.execution_stop_projection (operating_tenant_id, execution_id);

CREATE TABLE control_tower.execution_action_projection (
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    revision_id uuid NOT NULL,
    execution_stop_id uuid NOT NULL,
    action_id uuid NOT NULL,
    action_type text NOT NULL,
    shipment_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    status text NOT NULL,
    last_event_sequence bigint NOT NULL CHECK (last_event_sequence >= 0),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (execution_id, revision_id, action_id)
);

CREATE INDEX idx_execution_action_projection_shipment
    ON control_tower.execution_action_projection (shipment_id);

CREATE TABLE control_tower.execution_progress_event (
    operating_tenant_id uuid NOT NULL,
    event_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    revision_id uuid NULL,
    execution_stop_id uuid NULL,
    action_id uuid NULL,
    event_type text NOT NULL,
    shipment_id uuid NULL,
    reason_code text NULL,
    severity text NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (operating_tenant_id, event_id)
);

CREATE INDEX idx_execution_progress_event_execution
    ON control_tower.execution_progress_event (execution_id, occurred_at);
