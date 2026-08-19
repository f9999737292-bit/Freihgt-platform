CREATE TABLE billing.freight_settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    shipment_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    award_link_id UUID,
    settlement_number VARCHAR(100) NOT NULL,
    base_freight_amount NUMERIC(18,2) NOT NULL,
    currency_code CHAR(3) NOT NULL DEFAULT 'RUB',
    vat_rate NUMERIC(5,2),
    approved_accessorial_total NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_without_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_with_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    service_accepted_at TIMESTAMPTZ,
    service_accepted_by UUID,
    billing_register_id UUID REFERENCES billing.billing_registers(id),
    billing_register_item_id UUID,
    idempotency_key VARCHAR(128),
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_freight_settlement_shipment UNIQUE (tenant_id, shipment_id),
    CONSTRAINT uq_freight_settlement_number UNIQUE (tenant_id, settlement_number),
    CONSTRAINT chk_freight_settlement_status CHECK (
        status IN ('DRAFT','UNDER_REVIEW','DISPUTED','APPROVED','DOCUMENTS_READY','READY_FOR_PAYMENT','CANCELLED')
    )
);

CREATE UNIQUE INDEX uq_freight_settlement_idempotency
    ON billing.freight_settlements(tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_freight_settlements_tenant ON billing.freight_settlements(tenant_id);
CREATE INDEX idx_freight_settlements_buyer ON billing.freight_settlements(buyer_company_id);
CREATE INDEX idx_freight_settlements_carrier ON billing.freight_settlements(carrier_company_id);
CREATE INDEX idx_freight_settlements_status ON billing.freight_settlements(status);
CREATE INDEX idx_freight_settlements_shipment ON billing.freight_settlements(shipment_id);

CREATE TABLE billing.settlement_accessorials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    settlement_id UUID NOT NULL REFERENCES billing.freight_settlements(id) ON DELETE CASCADE,
    charge_code VARCHAR(50) NOT NULL,
    description TEXT,
    amount NUMERIC(18,2) NOT NULL,
    currency_code CHAR(3) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'PROPOSED',
    submitted_by UUID NOT NULL,
    submitted_by_company_id UUID NOT NULL,
    evidence_document_id UUID,
    evidence_type VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_settlement_accessorial_status CHECK (
        status IN ('PROPOSED','APPROVED','REJECTED','DISPUTED')
    ),
    CONSTRAINT chk_settlement_accessorial_amount CHECK (amount >= 0)
);

CREATE INDEX idx_settlement_accessorials_settlement ON billing.settlement_accessorials(settlement_id);
CREATE INDEX idx_settlement_accessorials_tenant ON billing.settlement_accessorials(tenant_id);
CREATE INDEX idx_settlement_accessorials_status ON billing.settlement_accessorials(status);

CREATE TABLE billing.settlement_disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    settlement_id UUID NOT NULL REFERENCES billing.freight_settlements(id) ON DELETE CASCADE,
    accessorial_id UUID REFERENCES billing.settlement_accessorials(id),
    reason TEXT NOT NULL,
    raised_by UUID NOT NULL,
    raised_by_company_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
    resolution_note TEXT,
    resolved_by UUID,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_settlement_dispute_status CHECK (status IN ('OPEN','RESOLVED','WITHDRAWN'))
);

CREATE INDEX idx_settlement_disputes_settlement ON billing.settlement_disputes(settlement_id);
CREATE INDEX idx_settlement_disputes_tenant ON billing.settlement_disputes(tenant_id);

CREATE TABLE billing.settlement_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    settlement_id UUID NOT NULL REFERENCES billing.freight_settlements(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    actor_user_id UUID,
    actor_company_id UUID,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_settlement_audit_settlement ON billing.settlement_audit_events(settlement_id);
CREATE INDEX idx_settlement_audit_tenant ON billing.settlement_audit_events(tenant_id);
