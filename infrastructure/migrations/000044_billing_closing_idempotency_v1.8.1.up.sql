CREATE UNIQUE INDEX uq_closing_package_per_register
    ON billing.closing_document_packages(tenant_id, register_id);

CREATE UNIQUE INDEX uq_invoice_per_register
    ON billing.invoices(tenant_id, register_id);

CREATE UNIQUE INDEX uq_act_per_register
    ON billing.acts(tenant_id, register_id);

CREATE UNIQUE INDEX uq_vat_invoice_per_register
    ON billing.vat_invoices(tenant_id, register_id);

CREATE UNIQUE INDEX uq_upd_per_register
    ON billing.upd_documents(tenant_id, register_id);
