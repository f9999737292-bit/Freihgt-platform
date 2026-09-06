-- RFx v3.0B — questionnaire definition core (versions, sections, questions, options, rules).

CREATE TABLE rfx.rfx_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'DRAFT',
    questionnaire_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    published_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT chk_rfx_version_status CHECK (
        status IN ('DRAFT', 'PUBLISHED', 'SUPERSEDED', 'ARCHIVED')
    ),
    CONSTRAINT chk_rfx_version_number CHECK (version_number > 0),
    CONSTRAINT uq_rfx_version_event_number UNIQUE (rfx_event_id, version_number)
);

CREATE INDEX idx_rfx_versions_tenant_id ON rfx.rfx_versions(tenant_id);
CREATE INDEX idx_rfx_versions_tenant_event_status ON rfx.rfx_versions(tenant_id, rfx_event_id, status);
CREATE INDEX idx_rfx_versions_event_id ON rfx.rfx_versions(rfx_event_id);

CREATE TABLE rfx.rfx_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_version_id UUID NOT NULL REFERENCES rfx.rfx_versions(id) ON DELETE CASCADE,
    section_code VARCHAR(100) NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_rfx_sections_version_code
    ON rfx.rfx_sections(rfx_version_id, section_code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_rfx_sections_tenant_id ON rfx.rfx_sections(tenant_id);
CREATE INDEX idx_rfx_sections_tenant_version_sort
    ON rfx.rfx_sections(tenant_id, rfx_version_id, sort_order);

CREATE TABLE rfx.rfx_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    section_id UUID NOT NULL REFERENCES rfx.rfx_sections(id) ON DELETE CASCADE,
    question_code VARCHAR(100) NOT NULL,
    question_type VARCHAR(50) NOT NULL,
    label TEXT NOT NULL,
    help_text TEXT,
    required BOOLEAN NOT NULL DEFAULT FALSE,
    validation_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_rfx_questions_section_code
    ON rfx.rfx_questions(section_id, question_code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_rfx_questions_tenant_id ON rfx.rfx_questions(tenant_id);
CREATE INDEX idx_rfx_questions_tenant_section_sort
    ON rfx.rfx_questions(tenant_id, section_id, sort_order);

CREATE TABLE rfx.rfx_question_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    question_id UUID NOT NULL REFERENCES rfx.rfx_questions(id) ON DELETE CASCADE,
    option_code VARCHAR(100) NOT NULL,
    label TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_rfx_question_options_question_code
    ON rfx.rfx_question_options(question_id, option_code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_rfx_question_options_tenant_id ON rfx.rfx_question_options(tenant_id);
CREATE INDEX idx_rfx_question_options_tenant_question_sort
    ON rfx.rfx_question_options(tenant_id, question_id, sort_order);

CREATE TABLE rfx.rfx_question_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_version_id UUID NOT NULL REFERENCES rfx.rfx_versions(id) ON DELETE CASCADE,
    target_question_id UUID REFERENCES rfx.rfx_questions(id) ON DELETE SET NULL,
    rule_code VARCHAR(100) NOT NULL,
    action VARCHAR(32) NOT NULL,
    condition_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT chk_rfx_question_rule_action CHECK (
        action IN ('SHOW', 'HIDE', 'REQUIRE')
    )
);

CREATE UNIQUE INDEX uq_rfx_question_rules_version_code
    ON rfx.rfx_question_rules(rfx_version_id, rule_code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_rfx_question_rules_tenant_id ON rfx.rfx_question_rules(tenant_id);
CREATE INDEX idx_rfx_question_rules_tenant_version_sort
    ON rfx.rfx_question_rules(tenant_id, rfx_version_id, sort_order);

ALTER TABLE rfx.rfx_events
    ADD COLUMN draft_version_id UUID REFERENCES rfx.rfx_versions(id) ON DELETE SET NULL;

CREATE INDEX idx_rfx_events_draft_version_id ON rfx.rfx_events(draft_version_id);
