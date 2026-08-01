CREATE TABLE transport.shipment_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    shipment_id UUID NOT NULL,
    shipment_version INTEGER NOT NULL,

    from_status VARCHAR(80),
    to_status VARCHAR(80) NOT NULL,

    reason_code VARCHAR(128),
    source VARCHAR(64) NOT NULL,

    actor_type VARCHAR(32) NOT NULL,
    actor_id UUID,

    correlation_id VARCHAR(128),

    occurred_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_shipment_status_history_shipment
        FOREIGN KEY (shipment_id)
        REFERENCES transport.shipments(id)
        ON DELETE CASCADE,

    CONSTRAINT uq_shipment_status_history_version
        UNIQUE (shipment_id, shipment_version),

    CONSTRAINT chk_shipment_status_history_actor_type CHECK (
        actor_type IN ('USER', 'SYSTEM')
    )
);

CREATE INDEX idx_shipment_status_history_tenant_shipment_time
    ON transport.shipment_status_history (tenant_id, shipment_id, occurred_at DESC);
