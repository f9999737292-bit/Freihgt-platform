-- Control Tower driver event integration v0.8.1

CREATE TABLE IF NOT EXISTS control_tower.driver_event_inbox (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES core.tenants(id),
    event_id            UUID NOT NULL,
    event_type          VARCHAR(128) NOT NULL,
    shipment_id         UUID,
    source_event_id     UUID,
    kafka_topic         VARCHAR(256),
    kafka_partition     INT,
    kafka_offset        BIGINT,
    payload_sha256      CHAR(64),
    processing_outcome  VARCHAR(32) NOT NULL,
    processed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_driver_event_inbox_tenant_event UNIQUE (tenant_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_driver_event_inbox_tenant_shipment
    ON control_tower.driver_event_inbox (tenant_id, shipment_id, processed_at DESC);

CREATE TABLE IF NOT EXISTS tracking.driver_tracking_automation_state (
    tenant_id                   UUID NOT NULL REFERENCES core.tenants(id),
    shipment_id                 UUID NOT NULL,
    automation_state            VARCHAR(32) NOT NULL DEFAULT 'TRACKING_OK',
    last_location_recorded_at   TIMESTAMPTZ,
    last_transition_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, shipment_id),
    CONSTRAINT chk_driver_tracking_automation_state
        CHECK (automation_state IN ('TRACKING_OK', 'TRACKING_LOST'))
);

CREATE INDEX IF NOT EXISTS idx_driver_tracking_automation_state_lost
    ON tracking.driver_tracking_automation_state (tenant_id, automation_state, last_location_recorded_at);

CREATE TABLE IF NOT EXISTS transport.driver_reported_delay (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES core.tenants(id),
    shipment_id         UUID NOT NULL REFERENCES transport.shipments(id),
    driver_id           UUID NOT NULL REFERENCES transport.drivers(id),
    reason_code         VARCHAR(64) NOT NULL,
    reason_text         TEXT,
    new_eta             TIMESTAMPTZ,
    occurred_at         TIMESTAMPTZ NOT NULL,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    idempotency_key     VARCHAR(128) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_driver_reported_delay_idempotency UNIQUE (tenant_id, driver_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_driver_reported_delay_shipment
    ON transport.driver_reported_delay (tenant_id, shipment_id, occurred_at DESC);
