DROP INDEX IF EXISTS billing.idx_freight_cost_outbox_tenant_aggregate;
DROP INDEX IF EXISTS billing.idx_freight_cost_outbox_pending;
DROP TABLE IF EXISTS billing.freight_cost_outbox;
ALTER TABLE billing.freight_settlements DROP COLUMN IF EXISTS billing_link_revision;
