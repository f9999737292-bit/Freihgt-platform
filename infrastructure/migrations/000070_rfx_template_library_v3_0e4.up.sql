-- RFx v3.0E4 — tenant-scoped template library aggregate and questionnaire graph.

CREATE TABLE IF NOT EXISTS rfx.rfx_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    template_code VARCHAR(128) NOT NULL,
    name_i18n_json JSONB NOT NULL,
    description_i18n_json JSONB NULL,
    rfx_type VARCHAR(32) NULL,
    owner_company_id UUID NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    version INT NOT NULL DEFAULT 1,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_rfx_templates_status CHECK (status IN ('ACTIVE', 'ARCHIVED')),
    CONSTRAINT chk_rfx_templates_version_positive CHECK (version > 0),
    CONSTRAINT chk_rfx_templates_name_i18n_object CHECK (jsonb_typeof(name_i18n_json) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_templates_tenant_code
    ON rfx.rfx_templates (tenant_id, template_code)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_templates_tenant_id
    ON rfx.rfx_templates (tenant_id, id);

CREATE INDEX IF NOT EXISTS idx_rfx_templates_tenant_status
    ON rfx.rfx_templates (tenant_id, status)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS rfx.rfx_template_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    template_id UUID NOT NULL,
    version_number INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'DRAFT',
    published_at TIMESTAMPTZ NULL,
    published_by UUID NULL,
    change_summary TEXT NULL,
    version INT NOT NULL DEFAULT 1,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_rfx_template_versions_status CHECK (status IN ('DRAFT', 'PUBLISHED', 'SUPERSEDED')),
    CONSTRAINT chk_rfx_template_versions_number CHECK (version_number > 0),
    CONSTRAINT uq_rfx_template_versions_template_number UNIQUE (template_id, version_number)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_versions_one_draft
    ON rfx.rfx_template_versions (template_id)
    WHERE status = 'DRAFT' AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_versions_one_published
    ON rfx.rfx_template_versions (template_id)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_rfx_template_versions_tenant_template
    ON rfx.rfx_template_versions (tenant_id, template_id, version_number DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_versions_tenant_template_id
    ON rfx.rfx_template_versions (tenant_id, template_id, id);

ALTER TABLE rfx.rfx_template_versions
    ADD CONSTRAINT fk_rfx_template_versions_template_composite
    FOREIGN KEY (tenant_id, template_id)
    REFERENCES rfx.rfx_templates (tenant_id, id);

CREATE TABLE IF NOT EXISTS rfx.rfx_template_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    template_id UUID NOT NULL,
    rfx_template_version_id UUID NOT NULL,
    section_code VARCHAR(100) NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    version INT NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_sections_version_code
    ON rfx.rfx_template_sections (rfx_template_version_id, section_code)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_rfx_template_sections_tenant_version
    ON rfx.rfx_template_sections (tenant_id, rfx_template_version_id, sort_order);

ALTER TABLE rfx.rfx_template_sections
    ADD CONSTRAINT fk_rfx_template_sections_version_composite
    FOREIGN KEY (tenant_id, template_id, rfx_template_version_id)
    REFERENCES rfx.rfx_template_versions (tenant_id, template_id, id);

CREATE TABLE IF NOT EXISTS rfx.rfx_template_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    section_id UUID NOT NULL,
    question_code VARCHAR(100) NOT NULL,
    question_type VARCHAR(50) NOT NULL,
    label TEXT NOT NULL,
    help_text TEXT NULL,
    required BOOLEAN NOT NULL DEFAULT FALSE,
    validation_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    version INT NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_questions_section_code
    ON rfx.rfx_template_questions (section_id, question_code)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_rfx_template_questions_tenant_section
    ON rfx.rfx_template_questions (tenant_id, section_id, sort_order);

ALTER TABLE rfx.rfx_template_questions
    ADD CONSTRAINT fk_rfx_template_questions_section
    FOREIGN KEY (section_id)
    REFERENCES rfx.rfx_template_sections (id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS rfx.rfx_template_question_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    question_id UUID NOT NULL,
    option_code VARCHAR(100) NOT NULL,
    label TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    version INT NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_question_options_question_code
    ON rfx.rfx_template_question_options (question_id, option_code)
    WHERE deleted_at IS NULL;

ALTER TABLE rfx.rfx_template_question_options
    ADD CONSTRAINT fk_rfx_template_question_options_question
    FOREIGN KEY (question_id)
    REFERENCES rfx.rfx_template_questions (id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS rfx.rfx_template_question_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    template_id UUID NOT NULL,
    rfx_template_version_id UUID NOT NULL,
    target_question_id UUID NULL,
    rule_code VARCHAR(100) NOT NULL,
    action VARCHAR(32) NOT NULL,
    condition_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT chk_rfx_template_question_rule_action CHECK (action IN ('SHOW', 'HIDE', 'REQUIRE'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_question_rules_version_code
    ON rfx.rfx_template_question_rules (rfx_template_version_id, rule_code)
    WHERE deleted_at IS NULL;

ALTER TABLE rfx.rfx_template_question_rules
    ADD CONSTRAINT fk_rfx_template_question_rules_version_composite
    FOREIGN KEY (tenant_id, template_id, rfx_template_version_id)
    REFERENCES rfx.rfx_template_versions (tenant_id, template_id, id);

ALTER TABLE rfx.rfx_template_question_rules
    ADD CONSTRAINT fk_rfx_template_question_rules_target_question
    FOREIGN KEY (target_question_id)
    REFERENCES rfx.rfx_template_questions (id) ON DELETE SET NULL;
