-- Control Tower Slot Intelligence v0.7.2

CREATE TABLE IF NOT EXISTS tracking.shipment_slot_revision (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    shipment_id             UUID NOT NULL,
    slot_type               VARCHAR(32) NOT NULL,
    facility_id             UUID,
    location_id             UUID,
    window_start            TIMESTAMPTZ NOT NULL,
    window_end              TIMESTAMPTZ NOT NULL,
    timezone                VARCHAR(64),
    slot_status             VARCHAR(16) NOT NULL,
    source_type             VARCHAR(32) NOT NULL,
    provider_code           VARCHAR(64),
    provider_slot_id        VARCHAR(128),
    provider_version        VARCHAR(64),
    dedup_key               VARCHAR(64) NOT NULL,
    source_observed_at      TIMESTAMPTZ NOT NULL,
    received_at             TIMESTAMPTZ NOT NULL,
    quality_status          VARCHAR(16) NOT NULL DEFAULT 'unknown',
    quality_reasons         JSONB NOT NULL DEFAULT '[]'::jsonb,
    booked_at               TIMESTAMPTZ,
    confirmed_at            TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_slot_revision_type CHECK (slot_type IN ('pickup', 'delivery')),
    CONSTRAINT chk_slot_revision_status CHECK (slot_status IN (
        'proposed', 'booked', 'confirmed', 'cancelled', 'completed', 'missed'
    )),
    CONSTRAINT chk_slot_revision_source CHECK (source_type IN (
        'internal_booking', 'warehouse_api', 'carrier_api', 'shipper_api', 'manual_operator', 'system_import'
    )),
    CONSTRAINT chk_slot_revision_quality CHECK (quality_status IN ('unknown', 'good', 'degraded')),
    CONSTRAINT chk_slot_revision_window CHECK (window_start < window_end)
);

CREATE UNIQUE INDEX uq_slot_revision_tenant_provider_slot
    ON tracking.shipment_slot_revision (tenant_id, provider_code, provider_slot_id, provider_version, slot_type)
    WHERE provider_slot_id IS NOT NULL AND provider_code IS NOT NULL AND provider_version IS NOT NULL;

CREATE UNIQUE INDEX uq_slot_revision_tenant_dedup
    ON tracking.shipment_slot_revision (tenant_id, dedup_key);

CREATE INDEX idx_slot_revision_tenant_shipment_type_observed
    ON tracking.shipment_slot_revision (tenant_id, shipment_id, slot_type, source_observed_at DESC);

CREATE INDEX idx_slot_revision_tenant_window_start
    ON tracking.shipment_slot_revision (tenant_id, window_start);

CREATE INDEX idx_slot_revision_tenant_window_end
    ON tracking.shipment_slot_revision (tenant_id, window_end);

CREATE INDEX idx_slot_revision_tenant_status
    ON tracking.shipment_slot_revision (tenant_id, slot_status);

CREATE INDEX idx_slot_revision_tenant_provider_slot_id
    ON tracking.shipment_slot_revision (tenant_id, provider_code, provider_slot_id)
    WHERE provider_slot_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS tracking.shipment_slot_state (
    tenant_id               UUID NOT NULL,
    shipment_id             UUID NOT NULL,
    slot_type               VARCHAR(32) NOT NULL,
    window_status           VARCHAR(16) NOT NULL DEFAULT 'unavailable',
    slot_status             VARCHAR(16),
    window_start            TIMESTAMPTZ,
    window_end              TIMESTAMPTZ,
    timezone                VARCHAR(64),
    facility_id             UUID,
    location_id             UUID,
    source_type             VARCHAR(32),
    provider_code           VARCHAR(64),
    provider_slot_id        VARCHAR(128),
    source_observed_at      TIMESTAMPTZ,
    received_at             TIMESTAMPTZ,
    quality_status          VARCHAR(16) NOT NULL DEFAULT 'unknown',
    booked_at               TIMESTAMPTZ,
    confirmed_at            TIMESTAMPTZ,
    version                 BIGINT NOT NULL DEFAULT 1,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, shipment_id, slot_type),
    CONSTRAINT chk_slot_state_window_status CHECK (window_status IN ('unavailable', 'available')),
    CONSTRAINT chk_slot_state_slot_status CHECK (slot_status IS NULL OR slot_status IN (
        'proposed', 'booked', 'confirmed', 'cancelled', 'completed', 'missed'
    )),
    CONSTRAINT chk_slot_state_quality CHECK (quality_status IN ('unknown', 'good', 'degraded')),
    CONSTRAINT chk_slot_state_window CHECK (
        window_start IS NULL OR window_end IS NULL OR window_start < window_end
    )
);

CREATE INDEX idx_shipment_slot_state_tenant_window_status
    ON tracking.shipment_slot_state (tenant_id, window_status);

CREATE INDEX idx_shipment_slot_state_tenant_slot_status
    ON tracking.shipment_slot_state (tenant_id, slot_status);

CREATE TABLE IF NOT EXISTS tracking.slot_state_transition (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    shipment_id         UUID NOT NULL,
    slot_type           VARCHAR(32) NOT NULL,
    transition_type     VARCHAR(32) NOT NULL,
    from_status         VARCHAR(16),
    to_status           VARCHAR(16) NOT NULL,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_slot_state_transition_type CHECK (transition_type IN (
        'slot_became_available', 'slot_rescheduled', 'slot_cancelled', 'slot_completed',
        'slot_missed', 'slot_projection_at_risk', 'slot_projection_miss', 'slot_projection_restored'
    ))
);

CREATE INDEX idx_slot_state_transition_tenant_shipment
    ON tracking.slot_state_transition (tenant_id, shipment_id, slot_type, occurred_at DESC);
