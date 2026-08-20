CREATE SCHEMA IF NOT EXISTS contract_rate;

CREATE TABLE contract_rate.transport_contract (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    contract_number VARCHAR(100) NOT NULL,
    external_reference VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'DRAFT',
    valid_from DATE NOT NULL,
    valid_to DATE,
    currency_code CHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    activated_at TIMESTAMPTZ,
    activated_by UUID,
    terminated_at TIMESTAMPTZ,
    terminated_by UUID,
    termination_reason TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_transport_contract_tenant_id UNIQUE (tenant_id, id),
    CONSTRAINT uq_transport_contract_number UNIQUE (tenant_id, buyer_company_id, contract_number),
    CONSTRAINT chk_transport_contract_status CHECK (
        status IN ('DRAFT', 'ACTIVE', 'SUSPENDED', 'TERMINATED', 'EXPIRED', 'CANCELLED')
    ),
    CONSTRAINT chk_transport_contract_dates CHECK (valid_to IS NULL OR valid_to >= valid_from),
    CONSTRAINT chk_transport_contract_currency CHECK (char_length(currency_code) = 3),
    CONSTRAINT chk_transport_contract_parties CHECK (buyer_company_id <> carrier_company_id)
);

CREATE INDEX idx_transport_contract_tenant ON contract_rate.transport_contract (tenant_id);
CREATE INDEX idx_transport_contract_buyer ON contract_rate.transport_contract (tenant_id, buyer_company_id);
CREATE INDEX idx_transport_contract_carrier ON contract_rate.transport_contract (tenant_id, carrier_company_id);
CREATE INDEX idx_transport_contract_status ON contract_rate.transport_contract (tenant_id, status);

CREATE TABLE contract_rate.rate_card (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    contract_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_rate_card_tenant_id UNIQUE (tenant_id, id),
    CONSTRAINT fk_rate_card_contract FOREIGN KEY (tenant_id, contract_id)
        REFERENCES contract_rate.transport_contract (tenant_id, id)
);

CREATE INDEX idx_rate_card_contract ON contract_rate.rate_card (tenant_id, contract_id);

CREATE TABLE contract_rate.rate_card_version (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rate_card_id UUID NOT NULL,
    version_number INTEGER NOT NULL,
    valid_from DATE NOT NULL,
    valid_to DATE,
    status VARCHAR(32) NOT NULL DEFAULT 'DRAFT',
    supersedes_version_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    activated_at TIMESTAMPTZ,
    activated_by UUID,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_rate_card_version_tenant_id UNIQUE (tenant_id, id),
    CONSTRAINT fk_rate_card_version_card FOREIGN KEY (tenant_id, rate_card_id)
        REFERENCES contract_rate.rate_card (tenant_id, id),
    CONSTRAINT fk_rate_card_version_supersedes FOREIGN KEY (tenant_id, supersedes_version_id)
        REFERENCES contract_rate.rate_card_version (tenant_id, id),
    CONSTRAINT uq_rate_card_version_number UNIQUE (rate_card_id, version_number),
    CONSTRAINT chk_rate_card_version_status CHECK (status IN ('DRAFT', 'ACTIVE', 'SUPERSEDED')),
    CONSTRAINT chk_rate_card_version_number CHECK (version_number > 0),
    CONSTRAINT chk_rate_card_version_dates CHECK (valid_to IS NULL OR valid_to >= valid_from)
);

CREATE UNIQUE INDEX uq_rate_card_version_one_active
    ON contract_rate.rate_card_version (rate_card_id)
    WHERE status = 'ACTIVE';

CREATE INDEX idx_rate_card_version_card ON contract_rate.rate_card_version (tenant_id, rate_card_id);

CREATE TABLE contract_rate.audit_event (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(64) NOT NULL,
    actor_user_id UUID,
    actor_company_id UUID,
    correlation_id VARCHAR(128),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_event_tenant_entity ON contract_rate.audit_event (tenant_id, entity_type, entity_id);
CREATE INDEX idx_audit_event_created_at ON contract_rate.audit_event (tenant_id, created_at DESC);
