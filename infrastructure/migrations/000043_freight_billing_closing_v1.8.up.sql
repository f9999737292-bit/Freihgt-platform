ALTER TABLE billing.billing_register_items
    ADD COLUMN settlement_id UUID REFERENCES billing.freight_settlements(id);

CREATE UNIQUE INDEX uq_billing_register_item_settlement
    ON billing.billing_register_items(tenant_id, settlement_id)
    WHERE settlement_id IS NOT NULL;

CREATE INDEX idx_billing_register_items_settlement
    ON billing.billing_register_items(settlement_id)
    WHERE settlement_id IS NOT NULL;

ALTER TABLE billing.freight_settlements
    ADD CONSTRAINT fk_freight_settlement_register_item
    FOREIGN KEY (billing_register_item_id)
    REFERENCES billing.billing_register_items(id);

CREATE TABLE billing.billing_register_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_id UUID NOT NULL REFERENCES billing.billing_registers(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    actor_user_id UUID,
    actor_company_id UUID,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_billing_register_audit_register ON billing.billing_register_audit_events(register_id);
CREATE INDEX idx_billing_register_audit_tenant ON billing.billing_register_audit_events(tenant_id);
