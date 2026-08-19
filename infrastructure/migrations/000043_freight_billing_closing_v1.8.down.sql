DROP TABLE IF EXISTS billing.billing_register_audit_events;

ALTER TABLE billing.freight_settlements
    DROP CONSTRAINT IF EXISTS fk_freight_settlement_register_item;

DROP INDEX IF EXISTS billing.idx_billing_register_items_settlement;
DROP INDEX IF EXISTS billing.uq_billing_register_item_settlement;

ALTER TABLE billing.billing_register_items
    DROP COLUMN IF EXISTS settlement_id;
