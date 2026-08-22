-- Option B: partial unique indexes preserve legacy paid semantics while allowing
-- multiple versioned paid_snapshot rows per obligation.

ALTER TABLE billing.payment_outbox
    ADD COLUMN aggregate_version BIGINT NOT NULL DEFAULT 0;

UPDATE billing.payment_outbox
SET aggregate_version = 0
WHERE event_type = 'payment_obligation.paid';

ALTER TABLE billing.payment_outbox
    DROP CONSTRAINT IF EXISTS uq_payment_outbox_tenant_event_aggregate;

CREATE UNIQUE INDEX uq_payment_outbox_legacy_paid
    ON billing.payment_outbox (tenant_id, event_type, aggregate_id)
    WHERE event_type = 'payment_obligation.paid';

CREATE UNIQUE INDEX uq_payment_outbox_paid_snapshot_version
    ON billing.payment_outbox (tenant_id, event_type, aggregate_id, aggregate_version)
    WHERE event_type = 'payment_obligation.paid_snapshot.v1';

CREATE INDEX idx_payment_outbox_tenant_aggregate_version
    ON billing.payment_outbox (tenant_id, aggregate_type, aggregate_id, aggregate_version);
