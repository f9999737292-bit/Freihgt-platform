CREATE SCHEMA IF NOT EXISTS freight_cost;

CREATE TABLE freight_cost.cost_entry (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    shipment_id UUID,
    buyer_company_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    entry_kind VARCHAR(64) NOT NULL,
    amount NUMERIC(18, 2),
    currency_code CHAR(3) NOT NULL,
    tax_basis VARCHAR(16) NOT NULL,
    amount_availability VARCHAR(16) NOT NULL,
    source_service VARCHAR(64) NOT NULL,
    source_type VARCHAR(64) NOT NULL,
    source_id UUID NOT NULL,
    source_revision BIGINT NOT NULL,
    source_fact_id UUID NOT NULL,
    source_event_id UUID NOT NULL,
    source_occurred_at TIMESTAMPTZ NOT NULL,
    supersedes_entry_id UUID,
    event_origin VARCHAR(32) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB,

    CONSTRAINT uq_cost_entry_tenant_source_event UNIQUE (tenant_id, source_event_id),
    CONSTRAINT uq_cost_entry_tenant_source_fact UNIQUE (tenant_id, source_fact_id),
    CONSTRAINT chk_cost_entry_amount_availability
        CHECK (amount_availability IN ('AVAILABLE', 'UNAVAILABLE')),
    CONSTRAINT chk_cost_entry_tax_basis
        CHECK (tax_basis IN ('EX_VAT', 'WITH_VAT')),
    CONSTRAINT chk_cost_entry_event_origin
        CHECK (event_origin IN ('LIVE_OUTBOX', 'CANONICAL_REBUILD')),
    CONSTRAINT chk_cost_entry_available_amount
        CHECK (amount_availability <> 'AVAILABLE' OR amount IS NOT NULL),
    CONSTRAINT chk_cost_entry_unavailable_amount
        CHECK (amount_availability <> 'UNAVAILABLE' OR amount IS NULL),
    CONSTRAINT chk_cost_entry_amount_nonneg
        CHECK (amount IS NULL OR amount >= 0),
    CONSTRAINT fk_cost_entry_supersedes
        FOREIGN KEY (supersedes_entry_id) REFERENCES freight_cost.cost_entry(id)
);

CREATE INDEX idx_cost_entry_tenant_to_recorded
    ON freight_cost.cost_entry (tenant_id, transport_order_id, recorded_at DESC);

CREATE INDEX idx_cost_entry_tenant_kind_to
    ON freight_cost.cost_entry (tenant_id, entry_kind, transport_order_id);

CREATE INDEX idx_cost_entry_source_lookup
    ON freight_cost.cost_entry (tenant_id, source_service, source_type, source_id, source_revision);

CREATE OR REPLACE FUNCTION freight_cost.deny_cost_entry_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'freight_cost.cost_entry is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_cost_entry_deny_update
    BEFORE UPDATE ON freight_cost.cost_entry
    FOR EACH ROW EXECUTE FUNCTION freight_cost.deny_cost_entry_mutation();

CREATE TRIGGER trg_cost_entry_deny_delete
    BEFORE DELETE ON freight_cost.cost_entry
    FOR EACH ROW EXECUTE FUNCTION freight_cost.deny_cost_entry_mutation();

CREATE TABLE freight_cost.source_cursor (
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    source_service VARCHAR(64) NOT NULL,
    source_type VARCHAR(64) NOT NULL,
    source_id UUID NOT NULL,
    entry_kind VARCHAR(64) NOT NULL,
    last_source_revision BIGINT NOT NULL DEFAULT 0,
    last_source_event_id UUID,
    last_cost_entry_id UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, transport_order_id, source_service, source_type, source_id, entry_kind),
    CONSTRAINT fk_source_cursor_last_entry
        FOREIGN KEY (last_cost_entry_id) REFERENCES freight_cost.cost_entry(id)
);

CREATE TABLE freight_cost.cost_summary_projection (
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    currency_code CHAR(3),
    planned_amount NUMERIC(18, 2),
    accrued_amount NUMERIC(18, 2),
    current_actual_amount NUMERIC(18, 2),
    final_actual_amount NUMERIC(18, 2),
    billing_register_amount NUMERIC(18, 2),
    payable_amount NUMERIC(18, 2),
    paid_amount NUMERIC(18, 2),
    billing_reconciliation_status VARCHAR(32) NOT NULL DEFAULT 'UNLINKED',
    financial_finality VARCHAR(32) NOT NULL DEFAULT 'NOT_EVALUATED',
    data_stage VARCHAR(64) NOT NULL DEFAULT 'PLANNED_ONLY',
    sources_available JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, transport_order_id)
);

CREATE INDEX idx_cost_projection_tenant_buyer
    ON freight_cost.cost_summary_projection (tenant_id, buyer_company_id);

CREATE INDEX idx_cost_projection_tenant_carrier
    ON freight_cost.cost_summary_projection (tenant_id, carrier_company_id);
