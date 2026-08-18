CREATE TABLE rfx.rfx_response_offer_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_response_id UUID NOT NULL REFERENCES rfx.rfx_responses(id) ON DELETE CASCADE,
    rfx_lot_id UUID REFERENCES rfx.rfx_lots(id) ON DELETE CASCADE,
    amount NUMERIC(18, 2) NOT NULL CHECK (amount >= 0),
    currency_code CHAR(3) NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version INTEGER NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_rfx_response_offer_line_lot
    ON rfx.rfx_response_offer_lines(rfx_response_id, rfx_lot_id)
    WHERE rfx_lot_id IS NOT NULL;

CREATE UNIQUE INDEX uq_rfx_response_offer_line_event
    ON rfx.rfx_response_offer_lines(rfx_response_id)
    WHERE rfx_lot_id IS NULL;

CREATE INDEX idx_rfx_response_offer_lines_response ON rfx.rfx_response_offer_lines(rfx_response_id);
CREATE INDEX idx_rfx_response_offer_lines_tenant ON rfx.rfx_response_offer_lines(tenant_id);

CREATE TABLE rfx.rfx_awards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    rfx_response_id UUID NOT NULL REFERENCES rfx.rfx_responses(id),
    carrier_company_id UUID NOT NULL,
    total_amount NUMERIC(18, 2),
    currency_code CHAR(3),
    awarded_by UUID,
    awarded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_rfx_award_event UNIQUE (rfx_event_id)
);

CREATE INDEX idx_rfx_awards_tenant ON rfx.rfx_awards(tenant_id);
CREATE INDEX idx_rfx_awards_response ON rfx.rfx_awards(rfx_response_id);

ALTER TABLE rfx.rfx_responses
    ADD COLUMN IF NOT EXISTS evaluation_rank INTEGER;

CREATE INDEX idx_rfx_audit_events_entity ON rfx.audit_events(tenant_id, entity_type, entity_id, created_at DESC);
