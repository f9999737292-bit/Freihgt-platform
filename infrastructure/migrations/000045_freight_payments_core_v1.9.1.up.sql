CREATE TABLE billing.payment_obligations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    obligation_number VARCHAR(100) NOT NULL,
    payer_company_id UUID NOT NULL,
    payee_company_id UUID NOT NULL,
    source_type VARCHAR(50) NOT NULL,
    source_id UUID NOT NULL,
    currency_code CHAR(3) NOT NULL,
    original_amount NUMERIC(18,2) NOT NULL,
    paid_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    outstanding_amount NUMERIC(18,2) NOT NULL,
    due_date DATE,
    status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
    blocked_reason VARCHAR(255),
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payment_obligation_number UNIQUE (tenant_id, obligation_number),
    CONSTRAINT uq_payment_obligation_source UNIQUE (tenant_id, source_type, source_id),
    CONSTRAINT chk_payment_obligation_status CHECK (
        status IN ('OPEN', 'PARTIALLY_PAID', 'PAID', 'CANCELLED', 'VOIDED')
    ),
    CONSTRAINT chk_payment_obligation_source_type CHECK (
        source_type IN ('BILLING_REGISTER')
    ),
    CONSTRAINT chk_payment_obligation_amounts CHECK (
        original_amount > 0
        AND paid_amount >= 0
        AND outstanding_amount >= 0
        AND paid_amount <= original_amount
        AND outstanding_amount = original_amount - paid_amount
    )
);

CREATE INDEX idx_payment_obligations_tenant_payer_status
    ON billing.payment_obligations(tenant_id, payer_company_id, status);
CREATE INDEX idx_payment_obligations_tenant_payee_status
    ON billing.payment_obligations(tenant_id, payee_company_id, status);
CREATE INDEX idx_payment_obligations_tenant_due_date
    ON billing.payment_obligations(tenant_id, due_date)
    WHERE status IN ('OPEN', 'PARTIALLY_PAID');

CREATE TABLE billing.payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    payment_number VARCHAR(100) NOT NULL,
    payer_company_id UUID NOT NULL,
    payee_company_id UUID NOT NULL,
    amount NUMERIC(18,2) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    payment_date DATE NOT NULL,
    value_date DATE,
    reference VARCHAR(255),
    external_reference VARCHAR(255),
    source VARCHAR(50) NOT NULL,
    external_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'RECEIVED',
    allocated_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    unallocated_amount NUMERIC(18,2) NOT NULL,
    created_by UUID NOT NULL,
    reconciled_at TIMESTAMPTZ,
    reconciled_by UUID,
    voided_at TIMESTAMPTZ,
    voided_by UUID,
    void_reason VARCHAR(255),
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payment_number UNIQUE (tenant_id, payment_number),
    CONSTRAINT chk_payment_status CHECK (
        status IN ('RECEIVED', 'PARTIALLY_ALLOCATED', 'FULLY_ALLOCATED', 'RECONCILED', 'VOIDED')
    ),
    CONSTRAINT chk_payment_source CHECK (
        source IN ('MANUAL', 'IMPORT', 'API', 'BANK_STATEMENT', 'BANK_API', 'ERP_1C', 'ERP_SAP')
    ),
    CONSTRAINT chk_payment_amounts CHECK (
        amount > 0
        AND allocated_amount >= 0
        AND unallocated_amount >= 0
        AND allocated_amount <= amount
        AND unallocated_amount = amount - allocated_amount
    )
);

CREATE UNIQUE INDEX uq_payment_bank_external_id
    ON billing.payments(tenant_id, source, external_id)
    WHERE external_id IS NOT NULL
      AND source IN ('IMPORT', 'API', 'BANK_STATEMENT', 'BANK_API', 'ERP_1C', 'ERP_SAP');

CREATE UNIQUE INDEX uq_payment_manual_external_id_active
    ON billing.payments(tenant_id, source, external_id)
    WHERE external_id IS NOT NULL AND source = 'MANUAL' AND voided_at IS NULL;

CREATE INDEX idx_payments_tenant_payer ON billing.payments(tenant_id, payer_company_id);
CREATE INDEX idx_payments_tenant_payee ON billing.payments(tenant_id, payee_company_id);
CREATE INDEX idx_payments_tenant_payment_date ON billing.payments(tenant_id, payment_date);
CREATE INDEX idx_payments_tenant_unallocated
    ON billing.payments(tenant_id, unallocated_amount)
    WHERE unallocated_amount > 0 AND voided_at IS NULL;

CREATE TABLE billing.payment_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    payment_id UUID NOT NULL REFERENCES billing.payments(id),
    obligation_id UUID NOT NULL REFERENCES billing.payment_obligations(id),
    allocated_amount NUMERIC(18,2) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    voided_at TIMESTAMPTZ,
    CONSTRAINT chk_payment_allocation_amount CHECK (allocated_amount > 0)
);

CREATE INDEX idx_payment_allocations_payment ON billing.payment_allocations(payment_id) WHERE voided_at IS NULL;
CREATE INDEX idx_payment_allocations_obligation ON billing.payment_allocations(obligation_id) WHERE voided_at IS NULL;

CREATE TABLE billing.payment_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    actor_user_id UUID,
    actor_company_id UUID,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_audit_entity ON billing.payment_audit_events(tenant_id, entity_type, entity_id, created_at DESC);

CREATE TRIGGER trg_payment_obligations_updated_at
BEFORE UPDATE ON billing.payment_obligations
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_payments_updated_at
BEFORE UPDATE ON billing.payments
FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();
