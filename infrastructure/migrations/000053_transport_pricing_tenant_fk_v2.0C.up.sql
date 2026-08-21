ALTER TABLE transport.transport_orders
    ADD CONSTRAINT uq_transport_orders_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE transport.transport_order_rate_snapshots
    ADD CONSTRAINT uq_transport_order_rate_snapshots_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE transport.transport_order_rate_snapshots
    DROP CONSTRAINT IF EXISTS transport_order_rate_snapshots_transport_order_id_fkey;

ALTER TABLE transport.transport_order_rate_snapshots
    ADD CONSTRAINT fk_transport_order_rate_snapshots_order_tenant
    FOREIGN KEY (tenant_id, transport_order_id)
    REFERENCES transport.transport_orders (tenant_id, id);

ALTER TABLE transport.transport_order_create_idempotency
    DROP CONSTRAINT IF EXISTS transport_order_create_idempotency_transport_order_id_fkey,
    DROP CONSTRAINT IF EXISTS transport_order_create_idempotency_rate_snapshot_id_fkey;

ALTER TABLE transport.transport_order_create_idempotency
    ADD CONSTRAINT fk_transport_order_create_idempotency_order_tenant
    FOREIGN KEY (tenant_id, transport_order_id)
    REFERENCES transport.transport_orders (tenant_id, id),
    ADD CONSTRAINT fk_transport_order_create_idempotency_snapshot_tenant
    FOREIGN KEY (tenant_id, rate_snapshot_id)
    REFERENCES transport.transport_order_rate_snapshots (tenant_id, id);
