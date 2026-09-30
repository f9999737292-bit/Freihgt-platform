-- TMS-MSTOP-0.1D execution-stop tracking.
-- Tracking owns route target state, execution-stop ETA state, the execution-event inbox,
-- and the tracking event outbox. Shipment execution rows are not copied here.

ALTER TABLE tracking.eta_observation
    ALTER COLUMN shipment_id DROP NOT NULL,
    ADD COLUMN execution_id uuid NULL,
    ADD COLUMN execution_stop_id uuid NULL;

ALTER TABLE tracking.eta_observation
    DROP CONSTRAINT chk_eta_observation_target_type;

ALTER TABLE tracking.eta_observation
    ADD CONSTRAINT chk_eta_observation_target_type
    CHECK (target_type IN ('pickup', 'delivery', 'execution_stop'));

ALTER TABLE tracking.eta_observation
    ADD CONSTRAINT chk_eta_observation_target_shape CHECK (
        (
            target_type IN ('pickup', 'delivery')
            AND shipment_id IS NOT NULL
            AND execution_stop_id IS NULL
        )
        OR (
            target_type = 'execution_stop'
            AND execution_id IS NOT NULL
            AND execution_stop_id IS NOT NULL
        )
    );

CREATE INDEX idx_eta_observation_execution_stop
    ON tracking.eta_observation (tenant_id, execution_stop_id, source_observed_at DESC)
    WHERE execution_stop_id IS NOT NULL;

CREATE TABLE tracking.execution_event_inbox (
    event_id uuid PRIMARY KEY,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    event_sequence bigint NOT NULL,
    event_type varchar(128) NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_execution_event_inbox_sequence UNIQUE (execution_id, event_sequence)
);

CREATE TABLE tracking.execution_tracking_state (
    operating_tenant_id uuid NOT NULL,
    execution_id uuid PRIMARY KEY,
    revision_id uuid NOT NULL,
    current_stop_id uuid NULL,
    current_stop_ordinal integer NULL,
    planned_arrival timestamptz NULL,
    location_id uuid NULL,
    target_latitude double precision NULL,
    target_longitude double precision NULL,
    live_eta_stop_id uuid NULL,
    live_eta_ordinal integer NULL,
    driver_id uuid NULL,
    vehicle_id uuid NULL,
    last_execution_event_sequence bigint NOT NULL,
    approach_emitted_stop_id uuid NULL,
    approach_emitted_at timestamptz NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_execution_tracking_coordinate_pair CHECK (
        (target_latitude IS NULL AND target_longitude IS NULL)
        OR (target_latitude IS NOT NULL AND target_longitude IS NOT NULL)
    )
);

CREATE INDEX idx_execution_tracking_state_driver
    ON tracking.execution_tracking_state (driver_id)
    WHERE driver_id IS NOT NULL AND live_eta_stop_id IS NOT NULL;

CREATE INDEX idx_execution_tracking_state_vehicle
    ON tracking.execution_tracking_state (vehicle_id)
    WHERE vehicle_id IS NOT NULL AND live_eta_stop_id IS NOT NULL;

CREATE TABLE tracking.execution_stop_eta_state (
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    execution_stop_id uuid NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'unavailable',
    estimated_arrival_at timestamptz NULL,
    source_type varchar(32) NULL,
    provider_code varchar(64) NULL,
    source_observed_at timestamptz NULL,
    received_at timestamptz NULL,
    freshness_status varchar(16) NOT NULL DEFAULT 'unknown',
    quality_status varchar(16) NOT NULL DEFAULT 'unknown',
    quality_reasons jsonb NOT NULL DEFAULT '[]'::jsonb,
    age_seconds bigint NULL,
    planned_arrival timestamptz NULL,
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (operating_tenant_id, execution_stop_id),
    CONSTRAINT chk_execution_stop_eta_state_status CHECK (status IN (
        'unavailable', 'available', 'stale', 'expired', 'completed'
    )),
    CONSTRAINT chk_execution_stop_eta_state_freshness CHECK (freshness_status IN (
        'unknown', 'fresh', 'stale', 'expired'
    )),
    CONSTRAINT chk_execution_stop_eta_state_quality CHECK (quality_status IN (
        'unknown', 'good', 'degraded', 'poor'
    ))
);

CREATE TABLE tracking.event_outbox (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    aggregate_type varchar(64) NOT NULL,
    aggregate_id uuid NOT NULL,
    aggregate_version integer NOT NULL,
    event_type varchar(128) NOT NULL,
    schema_version integer NOT NULL,
    source_event_id uuid NOT NULL,
    payload jsonb NOT NULL,
    headers jsonb NOT NULL DEFAULT '{}'::jsonb,
    status varchar(32) NOT NULL DEFAULT 'PENDING',
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz NULL,
    locked_by varchar(128) NULL,
    published_at timestamptz NULL,
    last_error_code varchar(128) NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_tracking_event_outbox_source_event UNIQUE (source_event_id),
    CONSTRAINT chk_tracking_event_outbox_status CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    CONSTRAINT chk_tracking_event_outbox_attempts CHECK (attempts >= 0)
);

CREATE INDEX idx_tracking_event_outbox_pending
    ON tracking.event_outbox (status, available_at, created_at)
    WHERE status = 'PENDING';
