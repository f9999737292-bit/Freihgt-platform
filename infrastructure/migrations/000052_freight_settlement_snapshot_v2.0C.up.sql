ALTER TABLE billing.freight_settlements
    ADD COLUMN IF NOT EXISTS rate_snapshot_id UUID,
    ADD COLUMN IF NOT EXISTS pricing_source VARCHAR(32);

CREATE INDEX idx_freight_settlements_rate_snapshot
    ON billing.freight_settlements(rate_snapshot_id)
    WHERE rate_snapshot_id IS NOT NULL;

COMMENT ON COLUMN billing.freight_settlements.rate_snapshot_id IS
    'Authoritative immutable TO rate snapshot for v2.0C+ orders';
COMMENT ON COLUMN billing.freight_settlements.pricing_source IS
    'Denormalized pricing source from TO snapshot when present';
