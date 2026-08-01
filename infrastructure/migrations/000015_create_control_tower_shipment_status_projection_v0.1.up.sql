CREATE SCHEMA IF NOT EXISTS control_tower;

CREATE TABLE control_tower.shipment_status_event_inbox (
    event_id UUID PRIMARY KEY,
    source_event_id UUID NOT NULL,

    tenant_id UUID NOT NULL,
    shipment_id UUID NOT NULL,
    aggregate_version INTEGER NOT NULL,

    event_type VARCHAR(128) NOT NULL,
    schema_version INTEGER NOT NULL,

    topic VARCHAR(249) NOT NULL,
    partition_id INTEGER NOT NULL,
    message_offset BIGINT NOT NULL,

    outcome VARCHAR(32) NOT NULL,

    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT uq_shipment_status_event_inbox_source_event
        UNIQUE (source_event_id),

    CONSTRAINT uq_shipment_status_event_inbox_position
        UNIQUE (topic, partition_id, message_offset),

    CONSTRAINT chk_shipment_status_event_inbox_version
        CHECK (aggregate_version > 0),

    CONSTRAINT chk_shipment_status_event_inbox_schema_version
        CHECK (schema_version > 0),

    CONSTRAINT chk_shipment_status_event_inbox_outcome
        CHECK (
            outcome IN (
                'APPLIED',
                'GAP_APPLIED',
                'STALE',
                'DUPLICATE'
            )
        )
);

CREATE TABLE control_tower.shipment_status_projection (
    tenant_id UUID NOT NULL,
    shipment_id UUID NOT NULL,

    shipment_version INTEGER NOT NULL,
    current_status VARCHAR(64) NOT NULL,
    previous_status VARCHAR(64),

    last_event_id UUID NOT NULL,
    last_source_event_id UUID NOT NULL,
    last_event_type VARCHAR(128) NOT NULL,

    last_occurred_at TIMESTAMPTZ NOT NULL,
    last_consumed_at TIMESTAMPTZ NOT NULL,

    complete BOOLEAN NOT NULL DEFAULT TRUE,
    gap_detected BOOLEAN NOT NULL DEFAULT FALSE,
    gap_from_version INTEGER,
    gap_to_version INTEGER,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, shipment_id),

    CONSTRAINT chk_shipment_status_projection_version
        CHECK (shipment_version > 0),

    CONSTRAINT chk_shipment_status_projection_gap
        CHECK (
            (gap_detected = FALSE
                AND gap_from_version IS NULL
                AND gap_to_version IS NULL)
            OR
            (gap_detected = TRUE
                AND gap_from_version IS NOT NULL
                AND gap_to_version IS NOT NULL
                AND gap_from_version <= gap_to_version)
        )
);

CREATE TABLE control_tower.shipment_status_event_dead_letter (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    topic VARCHAR(249) NOT NULL,
    partition_id INTEGER NOT NULL,
    message_offset BIGINT NOT NULL,

    event_id UUID,
    source_event_id UUID,
    tenant_id UUID,
    shipment_id UUID,
    schema_version INTEGER,

    payload_sha256 VARCHAR(64) NOT NULL,
    error_code VARCHAR(128) NOT NULL,

    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_shipment_status_event_dead_letter_position
        UNIQUE (topic, partition_id, message_offset)
);

CREATE INDEX idx_shipment_status_projection_tenant_status
    ON control_tower.shipment_status_projection (tenant_id, current_status);

CREATE INDEX idx_shipment_status_projection_tenant_updated
    ON control_tower.shipment_status_projection (tenant_id, updated_at DESC, shipment_id);
