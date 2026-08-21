DROP INDEX IF EXISTS billing.idx_freight_settlements_rate_snapshot;

ALTER TABLE billing.freight_settlements
    DROP COLUMN IF EXISTS pricing_source,
    DROP COLUMN IF EXISTS rate_snapshot_id;
