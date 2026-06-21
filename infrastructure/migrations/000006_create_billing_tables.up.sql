CREATE TABLE billing.billing_registers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_number VARCHAR(100) NOT NULL,
    customer_company_id UUID NOT NULL,
    contractor_company_id UUID NOT NULL,
    contract_id UUID,
    period_from DATE NOT NULL,
    period_to DATE NOT NULL,
    currency_code CHAR(3) NOT NULL DEFAULT 'RUB',
    vat_rate NUMERIC(5,2),
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    total_without_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_with_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    approved_at TIMESTAMPTZ,
    approved_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_billing_register_number UNIQUE (tenant_id, register_number),
    CONSTRAINT chk_billing_register_period CHECK (period_to >= period_from),
    CONSTRAINT chk_billing_register_status CHECK (
        status IN (
            'DRAFT','CALCULATED','UNDER_REVIEW','APPROVED',
            'CLOSING_DOCUMENTS_CREATED','SENT_TO_EDO','SIGNED_BY_COUNTERPARTY',
            'PAID','CLOSED','CANCELLED'
        )
    )
);

CREATE INDEX idx_billing_registers_tenant_id ON billing.billing_registers(tenant_id);
CREATE INDEX idx_billing_registers_customer ON billing.billing_registers(customer_company_id);
CREATE INDEX idx_billing_registers_contractor ON billing.billing_registers(contractor_company_id);
CREATE INDEX idx_billing_registers_status ON billing.billing_registers(status);
CREATE INDEX idx_billing_registers_period ON billing.billing_registers(period_from, period_to);

CREATE TABLE billing.billing_register_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_id UUID NOT NULL REFERENCES billing.billing_registers(id) ON DELETE CASCADE,
    shipment_id UUID NOT NULL,
    transport_order_id UUID,
    route_description TEXT,
    pickup_date DATE,
    delivery_date DATE,
    shipper_company_id UUID,
    consignee_company_id UUID,
    carrier_company_id UUID,
    base_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    extra_charges NUMERIC(18,2) NOT NULL DEFAULT 0,
    penalties NUMERIC(18,2) NOT NULL DEFAULT 0,
    amount_without_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    vat_rate NUMERIC(5,2),
    vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    amount_with_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_billing_register_shipment UNIQUE (register_id, shipment_id)
);

CREATE INDEX idx_billing_register_items_tenant_id ON billing.billing_register_items(tenant_id);
CREATE INDEX idx_billing_register_items_register_id ON billing.billing_register_items(register_id);
CREATE INDEX idx_billing_register_items_shipment_id ON billing.billing_register_items(shipment_id);
CREATE INDEX idx_billing_register_items_status ON billing.billing_register_items(status);

CREATE TABLE billing.closing_document_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_id UUID NOT NULL REFERENCES billing.billing_registers(id) ON DELETE CASCADE,
    package_number VARCHAR(100) NOT NULL,
    package_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_closing_document_package_number UNIQUE (tenant_id, package_number),
    CONSTRAINT chk_closing_document_package_type CHECK (
        package_type IN ('INVOICE_ONLY','ACT_PLUS_VAT_INVOICE','UPD','CUSTOM')
    )
);

CREATE INDEX idx_closing_document_packages_tenant_id ON billing.closing_document_packages(tenant_id);
CREATE INDEX idx_closing_document_packages_register_id ON billing.closing_document_packages(register_id);
CREATE INDEX idx_closing_document_packages_status ON billing.closing_document_packages(status);

CREATE TABLE billing.invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_id UUID NOT NULL REFERENCES billing.billing_registers(id) ON DELETE CASCADE,
    invoice_number VARCHAR(100) NOT NULL,
    invoice_date DATE NOT NULL,
    seller_company_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    total_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency_code CHAR(3) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    document_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_invoice_number UNIQUE (tenant_id, invoice_number)
);

CREATE INDEX idx_invoices_tenant_id ON billing.invoices(tenant_id);
CREATE INDEX idx_invoices_register_id ON billing.invoices(register_id);
CREATE INDEX idx_invoices_seller ON billing.invoices(seller_company_id);
CREATE INDEX idx_invoices_buyer ON billing.invoices(buyer_company_id);
CREATE INDEX idx_invoices_status ON billing.invoices(status);

CREATE TABLE billing.acts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_id UUID NOT NULL REFERENCES billing.billing_registers(id) ON DELETE CASCADE,
    act_number VARCHAR(100) NOT NULL,
    act_date DATE NOT NULL,
    seller_company_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    service_description TEXT,
    total_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency_code CHAR(3) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    document_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_act_number UNIQUE (tenant_id, act_number)
);

CREATE INDEX idx_acts_tenant_id ON billing.acts(tenant_id);
CREATE INDEX idx_acts_register_id ON billing.acts(register_id);
CREATE INDEX idx_acts_seller ON billing.acts(seller_company_id);
CREATE INDEX idx_acts_buyer ON billing.acts(buyer_company_id);

CREATE TABLE billing.vat_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_id UUID NOT NULL REFERENCES billing.billing_registers(id) ON DELETE CASCADE,
    vat_invoice_number VARCHAR(100) NOT NULL,
    vat_invoice_date DATE NOT NULL,
    seller_company_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    amount_without_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    vat_rate NUMERIC(5,2),
    vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    amount_with_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    document_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_vat_invoice_number UNIQUE (tenant_id, vat_invoice_number)
);

CREATE INDEX idx_vat_invoices_tenant_id ON billing.vat_invoices(tenant_id);
CREATE INDEX idx_vat_invoices_register_id ON billing.vat_invoices(register_id);
CREATE INDEX idx_vat_invoices_seller ON billing.vat_invoices(seller_company_id);
CREATE INDEX idx_vat_invoices_buyer ON billing.vat_invoices(buyer_company_id);

CREATE TABLE billing.upd_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    register_id UUID NOT NULL REFERENCES billing.billing_registers(id) ON DELETE CASCADE,
    upd_number VARCHAR(100) NOT NULL,
    upd_date DATE NOT NULL,
    seller_company_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    function_code VARCHAR(20) NOT NULL,
    amount_without_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    vat_rate NUMERIC(5,2),
    vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    amount_with_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    document_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    CONSTRAINT uq_upd_number UNIQUE (tenant_id, upd_number),
    CONSTRAINT chk_upd_function_code CHECK (function_code IN ('СЧФ','СЧФДОП','ДОП'))
);

CREATE INDEX idx_upd_documents_tenant_id ON billing.upd_documents(tenant_id);
CREATE INDEX idx_upd_documents_register_id ON billing.upd_documents(register_id);
CREATE INDEX idx_upd_documents_seller ON billing.upd_documents(seller_company_id);
CREATE INDEX idx_upd_documents_buyer ON billing.upd_documents(buyer_company_id);
CREATE INDEX idx_upd_documents_status ON billing.upd_documents(status);
