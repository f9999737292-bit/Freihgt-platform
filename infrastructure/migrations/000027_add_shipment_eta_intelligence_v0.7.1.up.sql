-- Control Tower ETA Intelligence v0.7.1

CREATE TABLE IF NOT EXISTS tracking.eta_observation (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    shipment_id             UUID NOT NULL,
    target_type             VARCHAR(32) NOT NULL,
    target_reference        VARCHAR(128),
    estimated_arrival_at    TIMESTAMPTZ NOT NULL,
    source_type             VARCHAR(32) NOT NULL,
    provider_code           VARCHAR(64),
    provider_event_id       VARCHAR(128),
    dedup_key               VARCHAR(64) NOT NULL,
    source_observed_at      TIMESTAMPTZ NOT NULL,
    received_at             TIMESTAMPTZ NOT NULL,
    quality_status          VARCHAR(16) NOT NULL DEFAULT 'unknown',
    quality_reasons         JSONB NOT NULL DEFAULT '[]'::jsonb,
    provider_confidence     NUMERIC(5, 4),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_eta_observation_target_type CHECK (target_type IN ('pickup', 'delivery')),
    CONSTRAINT chk_eta_observation_source_type CHECK (source_type IN (
        'provider_eta', 'carrier_eta', 'driver_eta', 'manual_operator', 'calculated'
    )),
    CONSTRAINT chk_eta_observation_quality_status CHECK (quality_status IN (
        'unknown', 'good', 'degraded', 'poor'
    ))
);

CREATE UNIQUE INDEX uq_eta_observation_tenant_provider_event
    ON tracking.eta_observation (tenant_id, provider_code, provider_event_id, target_type)
    WHERE provider_event_id IS NOT NULL AND provider_code IS NOT NULL;

CREATE UNIQUE INDEX uq_eta_observation_tenant_dedup
    ON tracking.eta_observation (tenant_id, dedup_key);

CREATE INDEX idx_eta_observation_tenant_shipment_target_observed
    ON tracking.eta_observation (tenant_id, shipment_id, target_type, source_observed_at DESC);

CREATE INDEX idx_eta_observation_tenant_target_arrival
    ON tracking.eta_observation (tenant_id, target_type, estimated_arrival_at DESC);

CREATE INDEX idx_eta_observation_tenant_provider_observed
    ON tracking.eta_observation (tenant_id, provider_code, source_observed_at DESC)
    WHERE provider_code IS NOT NULL;

CREATE TABLE IF NOT EXISTS tracking.shipment_eta_state (
    tenant_id               UUID NOT NULL,
    shipment_id             UUID NOT NULL,
    target_type             VARCHAR(32) NOT NULL,
    status                  VARCHAR(16) NOT NULL DEFAULT 'unavailable',
    estimated_arrival_at    TIMESTAMPTZ,
    source_type             VARCHAR(32),
    provider_code           VARCHAR(64),
    source_observed_at      TIMESTAMPTZ,
    received_at             TIMESTAMPTZ,
    freshness_status        VARCHAR(16) NOT NULL DEFAULT 'unknown',
    quality_status          VARCHAR(16) NOT NULL DEFAULT 'unknown',
    age_seconds             BIGINT,
    delivery_lag_seconds    BIGINT,
    version                 BIGINT NOT NULL DEFAULT 1,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, shipment_id, target_type),
    CONSTRAINT chk_shipment_eta_state_status CHECK (status IN (
        'unavailable', 'available', 'stale', 'expired', 'completed'
    )),
    CONSTRAINT chk_shipment_eta_state_freshness CHECK (freshness_status IN (
        'unknown', 'fresh', 'stale', 'expired'
    )),
    CONSTRAINT chk_shipment_eta_state_quality CHECK (quality_status IN (
        'unknown', 'good', 'degraded', 'poor'
    ))
);

CREATE INDEX idx_shipment_eta_state_tenant_status
    ON tracking.shipment_eta_state (tenant_id, status);

CREATE INDEX idx_shipment_eta_state_tenant_target
    ON tracking.shipment_eta_state (tenant_id, target_type, status);

CREATE TABLE IF NOT EXISTS tracking.eta_state_transition (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    shipment_id         UUID NOT NULL,
    target_type         VARCHAR(32) NOT NULL,
    transition_type     VARCHAR(32) NOT NULL,
    from_status         VARCHAR(16),
    to_status           VARCHAR(16) NOT NULL,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_eta_state_transition_type CHECK (transition_type IN (
        'eta_became_available', 'eta_became_stale', 'eta_expired', 'eta_restored', 'eta_completed', 'eta_source_changed'
    ))
);

CREATE INDEX idx_eta_state_transition_tenant_shipment
    ON tracking.eta_state_transition (tenant_id, shipment_id, target_type, occurred_at DESC);
