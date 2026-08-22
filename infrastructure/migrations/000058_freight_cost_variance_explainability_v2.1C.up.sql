CREATE TABLE freight_cost.charge_code_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mapping_scope VARCHAR(16) NOT NULL,
    tenant_id UUID,
    source_charge_code_normalized VARCHAR(50) NOT NULL,
    normalized_category VARCHAR(50) NOT NULL,
    mapping_version BIGINT NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_charge_code_mapping_scope
        CHECK (mapping_scope IN ('PLATFORM', 'TENANT')),
    CONSTRAINT chk_charge_code_mapping_tenant_scope
        CHECK (
            (mapping_scope = 'PLATFORM' AND tenant_id IS NULL)
            OR (mapping_scope = 'TENANT' AND tenant_id IS NOT NULL)
        )
);

CREATE UNIQUE INDEX uq_charge_code_mapping_active
    ON freight_cost.charge_code_mapping (
        mapping_scope,
        COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid),
        source_charge_code_normalized,
        effective_from
    );

CREATE INDEX idx_charge_code_mapping_lookup
    ON freight_cost.charge_code_mapping (mapping_scope, tenant_id, source_charge_code_normalized);

CREATE TABLE freight_cost.variance_attribution (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    attribution_fact_id UUID NOT NULL,
    semantic_class VARCHAR(32) NOT NULL,
    variance_kind VARCHAR(16) NOT NULL,
    reason_code VARCHAR(64) NOT NULL,
    evidence_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    mapping_version BIGINT NOT NULL DEFAULT 0,
    projection_revision BIGINT NOT NULL,
    is_current BOOLEAN NOT NULL DEFAULT TRUE,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_variance_attribution_fact UNIQUE (tenant_id, attribution_fact_id),
    CONSTRAINT chk_variance_attribution_class
        CHECK (semantic_class IN ('VARIANCE_DRIVER', 'VARIANCE_AVAILABILITY_REASON')),
    CONSTRAINT chk_variance_attribution_kind
        CHECK (variance_kind IN ('CURRENT', 'FINAL'))
);

CREATE INDEX idx_variance_attribution_to
    ON freight_cost.variance_attribution (tenant_id, transport_order_id, variance_kind, is_current);

CREATE TABLE freight_cost.reconciliation_finding (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    finding_id UUID NOT NULL,
    finding_kind VARCHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'OPEN',
    expected_revision BIGINT,
    observed_revision BIGINT,
    canonical_reference_key VARCHAR(256) NOT NULL DEFAULT '',
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    reopen_count INT NOT NULL DEFAULT 0,

    CONSTRAINT uq_reconciliation_finding_id UNIQUE (tenant_id, finding_id),
    CONSTRAINT chk_reconciliation_finding_status
        CHECK (status IN ('OPEN', 'RESOLVED', 'REOPENED'))
);

CREATE INDEX idx_reconciliation_finding_to_status
    ON freight_cost.reconciliation_finding (tenant_id, transport_order_id, status);

-- Platform default charge code mappings (tenant_id IS NULL).
INSERT INTO freight_cost.charge_code_mapping (
    mapping_scope, tenant_id, source_charge_code_normalized, normalized_category, mapping_version, effective_from
) VALUES
    ('PLATFORM', NULL, 'DETENTION', 'DETENTION', 1, NOW()),
    ('PLATFORM', NULL, 'FUEL', 'FUEL', 1, NOW()),
    ('PLATFORM', NULL, 'WAITING', 'WAITING', 1, NOW()),
    ('PLATFORM', NULL, 'LUMPER', 'LUMPER', 1, NOW());
