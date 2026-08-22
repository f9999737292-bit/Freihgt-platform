DROP INDEX IF EXISTS billing.idx_payment_outbox_tenant_aggregate_version;
DROP INDEX IF EXISTS billing.uq_payment_outbox_paid_snapshot_version;
DROP INDEX IF EXISTS billing.uq_payment_outbox_legacy_paid;

ALTER TABLE billing.payment_outbox
    ADD CONSTRAINT uq_payment_outbox_tenant_event_aggregate
        UNIQUE (tenant_id, event_type, aggregate_id);

ALTER TABLE billing.payment_outbox
    DROP COLUMN IF EXISTS aggregate_version;
