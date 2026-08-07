CREATE TABLE transport.shipment_event_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    tenant_id UUID NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_version INTEGER NOT NULL,

    event_type VARCHAR(128) NOT NULL,
    schema_version INTEGER NOT NULL,

    source_event_id UUID NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,

    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    locked_at TIMESTAMPTZ,
    locked_by VARCHAR(128),

    published_at TIMESTAMPTZ,
    last_error_code VARCHAR(128),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_shipment_event_outbox_history
        FOREIGN KEY (source_event_id)
        REFERENCES transport.shipment_status_history(id)
        ON DELETE CASCADE,

    CONSTRAINT uq_shipment_event_outbox_source_event
        UNIQUE (source_event_id),

    CONSTRAINT chk_shipment_event_outbox_status
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),

    CONSTRAINT chk_shipment_event_outbox_attempts
        CHECK (attempts >= 0)
);

CREATE INDEX idx_shipment_event_outbox_pending
    ON transport.shipment_event_outbox (status, available_at, created_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_shipment_event_outbox_tenant_aggregate
    ON transport.shipment_event_outbox (tenant_id, aggregate_id, aggregate_version);
