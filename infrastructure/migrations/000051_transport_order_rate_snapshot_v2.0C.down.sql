DROP TRIGGER IF EXISTS trg_transport_order_rate_snapshot_no_delete ON transport.transport_order_rate_snapshots;
DROP TRIGGER IF EXISTS trg_transport_order_rate_snapshot_no_update ON transport.transport_order_rate_snapshots;
DROP FUNCTION IF EXISTS transport.deny_transport_order_rate_snapshot_mutation();

DROP TABLE IF EXISTS transport.transport_order_create_idempotency;
DROP TABLE IF EXISTS transport.transport_order_rate_snapshots;

ALTER TABLE transport.transport_orders DROP COLUMN IF EXISTS pricing_model_version;
