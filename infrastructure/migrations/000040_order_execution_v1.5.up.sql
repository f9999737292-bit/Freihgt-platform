-- Idempotent shipment creation per transport order (one active shipment per order).
CREATE UNIQUE INDEX uq_shipments_transport_order_active
    ON transport.shipments (tenant_id, transport_order_id)
    WHERE deleted_at IS NULL AND transport_order_id IS NOT NULL;
