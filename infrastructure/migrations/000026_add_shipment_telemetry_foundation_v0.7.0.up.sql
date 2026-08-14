-- Control Tower Telemetry Foundation v0.7.0

CREATE SCHEMA IF NOT EXISTS tracking;

CREATE TABLE IF NOT EXISTS tracking.shipment_tracking_binding (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    shipment_id         UUID NOT NULL,
    vehicle_id          UUID,
    driver_id           UUID,
    provider_code       VARCHAR(64) NOT NULL,
    provider_device_id  VARCHAR(128) NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'active',
    active_from         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    active_to           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_shipment_tracking_binding_status CHECK (status IN ('active', 'inactive', 'revoked')),
    CONSTRAINT chk_shipment_tracking_binding_active_window CHECK (active_to IS NULL OR active_to >= active_from)
);

CREATE INDEX idx_shipment_tracking_binding_tenant_shipment
    ON tracking.shipment_tracking_binding (tenant_id, shipment_id, status);

CREATE INDEX idx_shipment_tracking_binding_tenant_provider_device
    ON tracking.shipment_tracking_binding (tenant_id, provider_code, provider_device_id, status);

CREATE UNIQUE INDEX uq_shipment_tracking_binding_active_device
    ON tracking.shipment_tracking_binding (tenant_id, provider_code, provider_device_id)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS tracking.location_event (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    shipment_id         UUID NOT NULL,
    vehicle_id          UUID,
    driver_id           UUID,
    provider_code       VARCHAR(64) NOT NULL,
    provider_device_id  VARCHAR(128) NOT NULL,
    provider_event_id   VARCHAR(128),
    dedup_key           VARCHAR(64) NOT NULL,
    latitude            NUMERIC(10, 7) NOT NULL,
    longitude           NUMERIC(10, 7) NOT NULL,
    recorded_at         TIMESTAMPTZ NOT NULL,
    received_at         TIMESTAMPTZ NOT NULL,
    speed_kph           NUMERIC(8, 2),
    heading_degrees     NUMERIC(6, 2),
    accuracy_meters     NUMERIC(10, 2),
    altitude_meters     NUMERIC(10, 2),
    source_type         VARCHAR(32) NOT NULL,
    quality_status      VARCHAR(16) NOT NULL DEFAULT 'unknown',
    quality_reason      VARCHAR(256),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_location_event_latitude CHECK (latitude >= -90 AND latitude <= 90),
    CONSTRAINT chk_location_event_longitude CHECK (longitude >= -180 AND longitude <= 180),
    CONSTRAINT chk_location_event_source_type CHECK (source_type IN (
        'vehicle_telematics', 'driver_mobile', 'carrier_api', 'manual', 'system_import'
    )),
    CONSTRAINT chk_location_event_quality_status CHECK (quality_status IN (
        'unknown', 'good', 'degraded', 'poor'
    ))
);

CREATE UNIQUE INDEX uq_location_event_tenant_provider_event
    ON tracking.location_event (tenant_id, provider_code, provider_event_id)
    WHERE provider_event_id IS NOT NULL;

CREATE UNIQUE INDEX uq_location_event_tenant_dedup
    ON tracking.location_event (tenant_id, dedup_key);

CREATE INDEX idx_location_event_tenant_shipment_recorded
    ON tracking.location_event (tenant_id, shipment_id, recorded_at DESC);

CREATE INDEX idx_location_event_tenant_provider_device_recorded
    ON tracking.location_event (tenant_id, provider_code, provider_device_id, recorded_at DESC);

CREATE INDEX idx_location_event_tenant_received
    ON tracking.location_event (tenant_id, received_at DESC);

CREATE TABLE IF NOT EXISTS tracking.shipment_tracking_state (
    tenant_id               UUID NOT NULL,
    shipment_id             UUID NOT NULL,
    tracking_status         VARCHAR(24) NOT NULL DEFAULT 'not_configured',
    provider_code           VARCHAR(64),
    last_latitude           NUMERIC(10, 7),
    last_longitude          NUMERIC(10, 7),
    last_recorded_at        TIMESTAMPTZ,
    last_received_at        TIMESTAMPTZ,
    last_speed_kph          NUMERIC(8, 2),
    last_heading_degrees    NUMERIC(6, 2),
    freshness_status        VARCHAR(16) NOT NULL DEFAULT 'unknown',
    quality_status          VARCHAR(16) NOT NULL DEFAULT 'unknown',
    age_seconds             BIGINT,
    delivery_delay_seconds  BIGINT,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, shipment_id),
    CONSTRAINT chk_shipment_tracking_state_status CHECK (tracking_status IN (
        'not_configured', 'awaiting_data', 'active', 'stale', 'lost', 'ended'
    )),
    CONSTRAINT chk_shipment_tracking_state_freshness CHECK (freshness_status IN (
        'unknown', 'fresh', 'stale', 'lost'
    )),
    CONSTRAINT chk_shipment_tracking_state_quality CHECK (quality_status IN (
        'unknown', 'good', 'degraded', 'poor'
    ))
);

CREATE INDEX idx_shipment_tracking_state_tenant_status
    ON tracking.shipment_tracking_state (tenant_id, tracking_status);

CREATE TABLE IF NOT EXISTS tracking.tracking_state_transition (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    shipment_id         UUID NOT NULL,
    transition_type     VARCHAR(32) NOT NULL,
    from_status         VARCHAR(24),
    to_status           VARCHAR(24) NOT NULL,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_tracking_state_transition_type CHECK (transition_type IN (
        'tracking_started', 'tracking_became_stale', 'tracking_lost', 'tracking_restored',
        'tracking_ended', 'binding_changed'
    ))
);

CREATE INDEX idx_tracking_state_transition_tenant_shipment
    ON tracking.tracking_state_transition (tenant_id, shipment_id, occurred_at DESC);
