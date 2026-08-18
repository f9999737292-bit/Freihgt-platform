CREATE TABLE rfx.rfx_award_transport_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id),
    rfx_award_id UUID NOT NULL REFERENCES rfx.rfx_awards(id),
    rfx_response_id UUID NOT NULL REFERENCES rfx.rfx_responses(id),
    rfx_lot_id UUID REFERENCES rfx.rfx_lots(id),
    rfx_lane_id UUID REFERENCES rfx.rfx_lanes(id),
    transport_order_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    amount NUMERIC(18, 2) NOT NULL CHECK (amount >= 0),
    currency_code CHAR(3) NOT NULL,
    converted_by UUID,
    converted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version INTEGER NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_rfx_award_transport_order_scope_lot
    ON rfx.rfx_award_transport_orders (tenant_id, rfx_event_id, rfx_lot_id)
    WHERE rfx_lot_id IS NOT NULL;

CREATE UNIQUE INDEX uq_rfx_award_transport_order_scope_event
    ON rfx.rfx_award_transport_orders (tenant_id, rfx_event_id)
    WHERE rfx_lot_id IS NULL;

CREATE INDEX idx_rfx_award_transport_orders_event ON rfx.rfx_award_transport_orders(rfx_event_id);
CREATE INDEX idx_rfx_award_transport_orders_order ON rfx.rfx_award_transport_orders(transport_order_id);
