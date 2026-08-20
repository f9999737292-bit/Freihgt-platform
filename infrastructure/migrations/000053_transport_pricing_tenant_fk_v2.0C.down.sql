ALTER TABLE transport.transport_order_create_idempotency
    DROP CONSTRAINT IF EXISTS fk_transport_order_create_idempotency_snapshot_tenant,
    DROP CONSTRAINT IF EXISTS fk_transport_order_create_idempotency_order_tenant;

ALTER TABLE transport.transport_order_create_idempotency
    ADD CONSTRAINT transport_order_create_idempotency_transport_order_id_fkey
    FOREIGN KEY (transport_order_id) REFERENCES transport.transport_orders (id),
    ADD CONSTRAINT transport_order_create_idempotency_rate_snapshot_id_fkey
    FOREIGN KEY (rate_snapshot_id) REFERENCES transport.transport_order_rate_snapshots (id);

ALTER TABLE transport.transport_order_rate_snapshots
    DROP CONSTRAINT IF EXISTS fk_transport_order_rate_snapshots_order_tenant;

ALTER TABLE transport.transport_order_rate_snapshots
    ADD CONSTRAINT transport_order_rate_snapshots_transport_order_id_fkey
    FOREIGN KEY (transport_order_id) REFERENCES transport.transport_orders (id);

ALTER TABLE transport.transport_order_rate_snapshots
    DROP CONSTRAINT IF EXISTS uq_transport_order_rate_snapshots_tenant_id;

ALTER TABLE transport.transport_orders
    DROP CONSTRAINT IF EXISTS uq_transport_orders_tenant_id;
