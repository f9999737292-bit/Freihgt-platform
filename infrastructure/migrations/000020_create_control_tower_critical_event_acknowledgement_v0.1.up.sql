CREATE TABLE IF NOT EXISTS control_tower.critical_event_acknowledgement (
    tenant_id UUID NOT NULL,
    event_id VARCHAR(32) NOT NULL,
    shipment_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'control-tower',
    occurred_at TIMESTAMPTZ NOT NULL,
    acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_by_user_id UUID NOT NULL,

    PRIMARY KEY (tenant_id, event_id),

    CONSTRAINT chk_critical_event_ack_event_id_format
        CHECK (event_id ~ '^[0-9a-f]{32}$'),

    CONSTRAINT chk_critical_event_ack_source
        CHECK (source = 'control-tower')
);

CREATE INDEX idx_critical_event_ack_tenant_shipment
    ON control_tower.critical_event_acknowledgement (tenant_id, shipment_id);
