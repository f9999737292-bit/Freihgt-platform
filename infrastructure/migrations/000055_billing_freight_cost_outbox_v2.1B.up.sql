ALTER TABLE billing.freight_settlements
    ADD COLUMN IF NOT EXISTS billing_link_revision BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN billing.freight_settlements.billing_link_revision IS
    'Monotonic revision for FREIGHT_SETTLEMENT_BILLING_LINK financial facts; increments on include/remove/relink only.';

UPDATE billing.freight_settlements
SET billing_link_revision = 1
WHERE billing_register_id IS NOT NULL
  AND billing_link_revision = 0;

CREATE TABLE billing.freight_cost_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    source_revision BIGINT NOT NULL,
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

    CONSTRAINT uq_freight_cost_outbox_tenant_event_revision
        UNIQUE (tenant_id, event_type, aggregate_id, source_revision),
    CONSTRAINT chk_freight_cost_outbox_status
        CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED')),
    CONSTRAINT chk_freight_cost_outbox_attempts
        CHECK (attempts >= 0)
);

CREATE INDEX idx_freight_cost_outbox_pending
    ON billing.freight_cost_outbox (status, available_at, created_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_freight_cost_outbox_tenant_aggregate
    ON billing.freight_cost_outbox (tenant_id, aggregate_type, aggregate_id, source_revision);
