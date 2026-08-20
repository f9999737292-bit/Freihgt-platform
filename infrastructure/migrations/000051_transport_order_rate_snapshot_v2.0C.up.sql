ALTER TABLE transport.transport_orders
    ADD COLUMN IF NOT EXISTS pricing_model_version VARCHAR(32);

COMMENT ON COLUMN transport.transport_orders.pricing_model_version IS
    'NULL = legacy pre-v2.0C; SNAPSHOT_V1 = immutable rate snapshot required';

CREATE TABLE transport.transport_order_rate_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL REFERENCES transport.transport_orders(id),

    buyer_company_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,

    pricing_source VARCHAR(32) NOT NULL,

    award_link_id UUID,
    rfx_event_id UUID,
    rfx_lot_id UUID,
    bid_id UUID,
    manual_spot_audit_id UUID,

    contract_id UUID,
    rate_card_id UUID,
    rate_version_id UUID,
    rate_line_id UUID,

    contract_number VARCHAR(100),
    rate_card_name VARCHAR(255),
    rate_version_number INTEGER,

    origin_location_id UUID NOT NULL,
    destination_location_id UUID NOT NULL,
    equipment_type VARCHAR(100) NOT NULL,
    transport_mode VARCHAR(32) NOT NULL,

    currency_code CHAR(3) NOT NULL,
    component_breakdown_status VARCHAR(32) NOT NULL,

    components JSONB NOT NULL DEFAULT '[]'::jsonb,
    accessorial_rules JSONB NOT NULL DEFAULT '[]'::jsonb,

    base_amount NUMERIC(18,2),
    total_amount NUMERIC(18,2) NOT NULL,

    pricing_date DATE NOT NULL,
    resolved_at TIMESTAMPTZ NOT NULL,
    resolved_by_service VARCHAR(64) NOT NULL,
    resolver_version VARCHAR(32) NOT NULL,
    resolution_request_hash CHAR(64) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_transport_order_rate_snapshot UNIQUE (tenant_id, transport_order_id),
    CONSTRAINT chk_snapshot_pricing_source CHECK (
        pricing_source IN ('RFQ_AWARD', 'SPOT_BID', 'CONTRACT_RATE', 'MANUAL_SPOT')
    ),
    CONSTRAINT chk_snapshot_breakdown_status CHECK (
        component_breakdown_status IN ('AVAILABLE', 'UNAVAILABLE')
    ),
    CONSTRAINT chk_snapshot_total_nonneg CHECK (total_amount >= 0),
    CONSTRAINT chk_snapshot_available_invariant CHECK (
        component_breakdown_status <> 'AVAILABLE'
        OR (base_amount IS NOT NULL AND jsonb_array_length(components) > 0)
    ),
    CONSTRAINT chk_snapshot_unavailable_invariant CHECK (
        component_breakdown_status <> 'UNAVAILABLE'
        OR (base_amount IS NULL AND components = '[]'::jsonb)
    ),
    CONSTRAINT chk_snapshot_rfq_award_provenance CHECK (
        pricing_source <> 'RFQ_AWARD'
        OR award_link_id IS NOT NULL
        OR rfx_event_id IS NOT NULL
    ),
    CONSTRAINT chk_snapshot_spot_bid_provenance CHECK (
        pricing_source <> 'SPOT_BID' OR bid_id IS NOT NULL
    ),
    CONSTRAINT chk_snapshot_manual_provenance CHECK (
        pricing_source <> 'MANUAL_SPOT' OR manual_spot_audit_id IS NOT NULL
    ),
    CONSTRAINT chk_snapshot_contract_provenance CHECK (
        pricing_source <> 'CONTRACT_RATE'
        OR (contract_id IS NOT NULL AND rate_card_id IS NOT NULL AND rate_version_id IS NOT NULL AND rate_line_id IS NOT NULL)
    )
);

CREATE INDEX idx_transport_order_rate_snapshots_tenant
    ON transport.transport_order_rate_snapshots(tenant_id);
CREATE INDEX idx_transport_order_rate_snapshots_order
    ON transport.transport_order_rate_snapshots(transport_order_id);

CREATE TABLE transport.transport_order_create_idempotency (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    actor_company_id UUID NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    request_hash CHAR(64) NOT NULL,
    transport_order_id UUID NOT NULL REFERENCES transport.transport_orders(id),
    rate_snapshot_id UUID NOT NULL REFERENCES transport.transport_order_rate_snapshots(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_transport_order_create_idempotency UNIQUE (tenant_id, actor_company_id, idempotency_key)
);

CREATE INDEX idx_transport_order_create_idempotency_order
    ON transport.transport_order_create_idempotency(transport_order_id);

CREATE OR REPLACE FUNCTION transport.deny_transport_order_rate_snapshot_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'transport_order_rate_snapshots is immutable';
END;
$$;

CREATE TRIGGER trg_transport_order_rate_snapshot_no_update
    BEFORE UPDATE ON transport.transport_order_rate_snapshots
    FOR EACH ROW EXECUTE FUNCTION transport.deny_transport_order_rate_snapshot_mutation();

CREATE TRIGGER trg_transport_order_rate_snapshot_no_delete
    BEFORE DELETE ON transport.transport_order_rate_snapshots
    FOR EACH ROW EXECUTE FUNCTION transport.deny_transport_order_rate_snapshot_mutation();
