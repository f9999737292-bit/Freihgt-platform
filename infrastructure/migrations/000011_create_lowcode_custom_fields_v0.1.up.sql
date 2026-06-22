CREATE SCHEMA IF NOT EXISTS lowcode;

CREATE TABLE lowcode.low_code_configurations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    config_type         TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'DRAFT',
    version             INTEGER NOT NULL DEFAULT 1,
    owner_company_id    UUID,
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at        TIMESTAMPTZ,
    archived_at         TIMESTAMPTZ,
    CONSTRAINT uq_lowcode_configurations_tenant_code_version
        UNIQUE (tenant_id, code, version),
    CONSTRAINT chk_lowcode_configurations_status
        CHECK (status IN ('DRAFT', 'REVIEW', 'PUBLISHED', 'ARCHIVED')),
    CONSTRAINT chk_lowcode_configurations_config_type
        CHECK (config_type IN (
            'FORM_TEMPLATE',
            'RULE_SET',
            'DOCUMENT_TEMPLATE',
            'DASHBOARD_TEMPLATE',
            'PROCESS_TEMPLATE',
            'CONNECTOR'
        ))
);

CREATE INDEX idx_lowcode_configurations_tenant_id
    ON lowcode.low_code_configurations (tenant_id);
CREATE INDEX idx_lowcode_configurations_tenant_code
    ON lowcode.low_code_configurations (tenant_id, code);
CREATE INDEX idx_lowcode_configurations_tenant_config_type
    ON lowcode.low_code_configurations (tenant_id, config_type);
CREATE INDEX idx_lowcode_configurations_tenant_status
    ON lowcode.low_code_configurations (tenant_id, status);
CREATE INDEX idx_lowcode_configurations_tenant_type_status
    ON lowcode.low_code_configurations (tenant_id, config_type, status);

CREATE TABLE lowcode.form_templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    configuration_id    UUID NOT NULL REFERENCES lowcode.low_code_configurations(id),
    entity_type         TEXT NOT NULL,
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    status              TEXT NOT NULL DEFAULT 'DRAFT',
    version             INTEGER NOT NULL DEFAULT 1,
    locale_default      TEXT NOT NULL DEFAULT 'ru-RU',
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at        TIMESTAMPTZ,
    CONSTRAINT uq_form_templates_tenant_entity_code_version
        UNIQUE (tenant_id, entity_type, code, version),
    CONSTRAINT chk_form_templates_status
        CHECK (status IN ('DRAFT', 'REVIEW', 'PUBLISHED', 'ARCHIVED')),
    CONSTRAINT chk_form_templates_entity_type
        CHECK (entity_type IN (
            'TRANSPORT_ORDER',
            'RFX',
            'FREIGHT_REQUEST',
            'BID',
            'SHIPMENT',
            'DOCUMENT',
            'BILLING_REGISTER',
            'COMPANY_PROFILE',
            'DRIVER_TASK'
        ))
);

CREATE INDEX idx_form_templates_tenant_id
    ON lowcode.form_templates (tenant_id);
CREATE INDEX idx_form_templates_tenant_entity_type
    ON lowcode.form_templates (tenant_id, entity_type);
CREATE INDEX idx_form_templates_tenant_status
    ON lowcode.form_templates (tenant_id, status);
CREATE INDEX idx_form_templates_tenant_entity_status
    ON lowcode.form_templates (tenant_id, entity_type, status);
CREATE INDEX idx_form_templates_configuration_id
    ON lowcode.form_templates (configuration_id);

CREATE TABLE lowcode.form_sections (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    form_template_id        UUID NOT NULL REFERENCES lowcode.form_templates(id) ON DELETE CASCADE,
    code                    TEXT NOT NULL,
    title                   TEXT NOT NULL,
    description             TEXT,
    sort_order              INTEGER NOT NULL DEFAULT 100,
    visibility_rule_json    JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_form_sections_tenant_template_code
        UNIQUE (tenant_id, form_template_id, code)
);

CREATE INDEX idx_form_sections_tenant_template
    ON lowcode.form_sections (tenant_id, form_template_id);
CREATE INDEX idx_form_sections_tenant_template_sort
    ON lowcode.form_sections (tenant_id, form_template_id, sort_order);

CREATE TABLE lowcode.form_fields (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    form_template_id        UUID NOT NULL REFERENCES lowcode.form_templates(id) ON DELETE CASCADE,
    section_id              UUID REFERENCES lowcode.form_sections(id) ON DELETE SET NULL,
    code                    TEXT NOT NULL,
    label                   TEXT NOT NULL,
    field_type              TEXT NOT NULL,
    required                BOOLEAN NOT NULL DEFAULT false,
    read_only               BOOLEAN NOT NULL DEFAULT false,
    system_field            BOOLEAN NOT NULL DEFAULT false,
    default_value_json      JSONB,
    options_json            JSONB,
    validation_rule_json    JSONB,
    visibility_rule_json    JSONB,
    localization_json       JSONB,
    sort_order              INTEGER NOT NULL DEFAULT 100,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_form_fields_tenant_template_code
        UNIQUE (tenant_id, form_template_id, code),
    CONSTRAINT chk_form_fields_field_type
        CHECK (field_type IN (
            'TEXT', 'NUMBER', 'DATE', 'DATETIME',
            'SELECT', 'MULTI_SELECT', 'CHECKBOX', 'FILE',
            'COMPANY_REFERENCE', 'ROUTE', 'ADDRESS', 'VEHICLE',
            'MONEY', 'CURRENCY', 'VAT_TAX', 'DOCUMENT_REFERENCE'
        ))
);

CREATE INDEX idx_form_fields_tenant_template
    ON lowcode.form_fields (tenant_id, form_template_id);
CREATE INDEX idx_form_fields_tenant_section
    ON lowcode.form_fields (tenant_id, section_id);
CREATE INDEX idx_form_fields_tenant_field_type
    ON lowcode.form_fields (tenant_id, field_type);
CREATE INDEX idx_form_fields_tenant_template_sort
    ON lowcode.form_fields (tenant_id, form_template_id, sort_order);

CREATE TABLE lowcode.custom_field_values (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    entity_type         TEXT NOT NULL,
    entity_id           UUID NOT NULL,
    form_template_id    UUID REFERENCES lowcode.form_templates(id),
    field_id            UUID NOT NULL REFERENCES lowcode.form_fields(id),
    field_code          TEXT NOT NULL,
    value_json          JSONB,
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_custom_field_values_tenant_entity_field
        UNIQUE (tenant_id, entity_type, entity_id, field_id),
    CONSTRAINT chk_custom_field_values_entity_type
        CHECK (entity_type IN (
            'TRANSPORT_ORDER',
            'RFX',
            'FREIGHT_REQUEST',
            'BID',
            'SHIPMENT',
            'DOCUMENT',
            'BILLING_REGISTER',
            'COMPANY_PROFILE',
            'DRIVER_TASK'
        ))
);

CREATE INDEX idx_custom_field_values_tenant_entity
    ON lowcode.custom_field_values (tenant_id, entity_type, entity_id);
CREATE INDEX idx_custom_field_values_tenant_field_id
    ON lowcode.custom_field_values (tenant_id, field_id);
CREATE INDEX idx_custom_field_values_tenant_field_code
    ON lowcode.custom_field_values (tenant_id, field_code);

CREATE TABLE lowcode.rule_sets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    configuration_id    UUID REFERENCES lowcode.low_code_configurations(id),
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    rule_set_type       TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'DRAFT',
    version             INTEGER NOT NULL DEFAULT 1,
    created_by_user_id  UUID,
    updated_by_user_id  UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at        TIMESTAMPTZ,
    CONSTRAINT uq_rule_sets_tenant_code_version
        UNIQUE (tenant_id, code, version),
    CONSTRAINT chk_rule_sets_status
        CHECK (status IN ('DRAFT', 'REVIEW', 'PUBLISHED', 'ARCHIVED')),
    CONSTRAINT chk_rule_sets_rule_set_type
        CHECK (rule_set_type IN (
            'VALIDATION', 'VISIBILITY', 'READ_ONLY', 'NOTIFICATION', 'APPROVAL'
        ))
);

CREATE INDEX idx_rule_sets_tenant_id
    ON lowcode.rule_sets (tenant_id);
CREATE INDEX idx_rule_sets_tenant_type
    ON lowcode.rule_sets (tenant_id, rule_set_type);
CREATE INDEX idx_rule_sets_tenant_status
    ON lowcode.rule_sets (tenant_id, status);

CREATE TABLE lowcode.rules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    rule_set_id         UUID NOT NULL REFERENCES lowcode.rule_sets(id) ON DELETE CASCADE,
    code                TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    priority            INTEGER NOT NULL DEFAULT 100,
    condition_json      JSONB NOT NULL,
    action_json         JSONB NOT NULL,
    enabled             BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rules_tenant_set_code
        UNIQUE (tenant_id, rule_set_id, code)
);

CREATE INDEX idx_rules_tenant_rule_set
    ON lowcode.rules (tenant_id, rule_set_id);
CREATE INDEX idx_rules_tenant_set_enabled
    ON lowcode.rules (tenant_id, rule_set_id, enabled);
CREATE INDEX idx_rules_tenant_set_priority
    ON lowcode.rules (tenant_id, rule_set_id, priority);

CREATE TABLE lowcode.configuration_audit_log (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    configuration_id    UUID,
    entity_type         TEXT NOT NULL,
    entity_id           UUID,
    action              TEXT NOT NULL,
    old_value_json      JSONB,
    new_value_json      JSONB,
    changed_by_user_id  UUID,
    request_id          TEXT,
    ip_address          TEXT,
    user_agent          TEXT,
    changed_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_configuration_audit_log_action
        CHECK (action IN (
            'CREATE', 'UPDATE', 'DELETE',
            'PUBLISH', 'ARCHIVE', 'ROLLBACK',
            'TEST', 'SIMULATE'
        ))
);

CREATE INDEX idx_configuration_audit_log_tenant_config
    ON lowcode.configuration_audit_log (tenant_id, configuration_id);
CREATE INDEX idx_configuration_audit_log_tenant_entity
    ON lowcode.configuration_audit_log (tenant_id, entity_type, entity_id);
CREATE INDEX idx_configuration_audit_log_tenant_changed_at
    ON lowcode.configuration_audit_log (tenant_id, changed_at DESC);
CREATE INDEX idx_configuration_audit_log_tenant_action
    ON lowcode.configuration_audit_log (tenant_id, action);

CREATE TABLE lowcode.configuration_approvals (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    configuration_id      UUID NOT NULL REFERENCES lowcode.low_code_configurations(id),
    requested_by_user_id    UUID,
    approved_by_user_id     UUID,
    status                  TEXT NOT NULL DEFAULT 'PENDING',
    comment                 TEXT,
    requested_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    approved_at             TIMESTAMPTZ,
    rejected_at             TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,
    CONSTRAINT chk_configuration_approvals_status
        CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'CANCELLED'))
);

CREATE INDEX idx_configuration_approvals_tenant_config
    ON lowcode.configuration_approvals (tenant_id, configuration_id);
CREATE INDEX idx_configuration_approvals_tenant_status
    ON lowcode.configuration_approvals (tenant_id, status);
CREATE INDEX idx_configuration_approvals_tenant_requested_at
    ON lowcode.configuration_approvals (tenant_id, requested_at DESC);

CREATE TRIGGER trg_lowcode_configurations_updated_at
    BEFORE UPDATE ON lowcode.low_code_configurations
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_lowcode_form_templates_updated_at
    BEFORE UPDATE ON lowcode.form_templates
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_lowcode_form_sections_updated_at
    BEFORE UPDATE ON lowcode.form_sections
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_lowcode_form_fields_updated_at
    BEFORE UPDATE ON lowcode.form_fields
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_lowcode_custom_field_values_updated_at
    BEFORE UPDATE ON lowcode.custom_field_values
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_lowcode_rule_sets_updated_at
    BEFORE UPDATE ON lowcode.rule_sets
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();

CREATE TRIGGER trg_lowcode_rules_updated_at
    BEFORE UPDATE ON lowcode.rules
    FOR EACH ROW EXECUTE FUNCTION core.set_updated_at();
