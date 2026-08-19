CREATE TABLE billing.payment_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    tenant_id UUID NOT NULL,

    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,

    event_type VARCHAR(128) NOT NULL,
    schema_version INTEGER NOT NULL DEFAULT 1,

    payload JSONB NOT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    locked_at TIMESTAMPTZ,
    locked_by VARCHAR(128),

    published_at TIMESTAMPTZ,
    last_error_code VARCHAR(128),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_payment_outbox_tenant_event_aggregate
        UNIQUE (tenant_id, event_type, aggregate_id),

    CONSTRAINT chk_payment_outbox_status
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),

    CONSTRAINT chk_payment_outbox_attempts
        CHECK (attempts >= 0)
);

CREATE INDEX idx_payment_outbox_pending
    ON billing.payment_outbox (status, available_at, created_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_payment_outbox_tenant_aggregate
    ON billing.payment_outbox (tenant_id, aggregate_type, aggregate_id);
