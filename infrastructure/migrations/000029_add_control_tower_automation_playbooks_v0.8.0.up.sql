-- Control Tower Automation Rules & Operational Playbooks v0.8.0

CREATE TABLE IF NOT EXISTS control_tower.automation_rule (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    name                VARCHAR(128) NOT NULL,
    description         TEXT,
    status              VARCHAR(16) NOT NULL DEFAULT 'draft',
    trigger_type        VARCHAR(64) NOT NULL,
    conditions          JSONB NOT NULL DEFAULT '{}'::jsonb,
    condition_schema_version SMALLINT NOT NULL DEFAULT 1,
    playbook_id         UUID,
    execution_mode      VARCHAR(16) NOT NULL DEFAULT 'recommend',
    priority            INTEGER NOT NULL DEFAULT 50,
    version             INTEGER NOT NULL DEFAULT 1,
    created_by_user_id  UUID NOT NULL,
    updated_by_user_id  UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_automation_rule_status CHECK (status IN ('draft', 'active', 'disabled', 'retired')),
    CONSTRAINT chk_automation_rule_execution_mode CHECK (execution_mode IN ('observe', 'recommend', 'guarded_auto')),
    CONSTRAINT chk_automation_rule_priority CHECK (priority >= 0 AND priority <= 1000)
);

CREATE INDEX idx_automation_rule_tenant_status_trigger
    ON control_tower.automation_rule (tenant_id, status, trigger_type);

CREATE INDEX idx_automation_rule_tenant_playbook
    ON control_tower.automation_rule (tenant_id, playbook_id);

CREATE TABLE IF NOT EXISTS control_tower.automation_rule_version (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    rule_id             UUID NOT NULL REFERENCES control_tower.automation_rule(id) ON DELETE CASCADE,
    version             INTEGER NOT NULL,
    name                VARCHAR(128) NOT NULL,
    description         TEXT,
    status              VARCHAR(16) NOT NULL,
    trigger_type        VARCHAR(64) NOT NULL,
    conditions          JSONB NOT NULL,
    condition_schema_version SMALLINT NOT NULL DEFAULT 1,
    playbook_id         UUID,
    execution_mode      VARCHAR(16) NOT NULL,
    priority            INTEGER NOT NULL,
    created_by_user_id  UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_automation_rule_version UNIQUE (tenant_id, rule_id, version)
);

CREATE INDEX idx_automation_rule_version_rule
    ON control_tower.automation_rule_version (rule_id, version DESC);

CREATE TABLE IF NOT EXISTS control_tower.operational_playbook (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    name                VARCHAR(128) NOT NULL,
    description         TEXT,
    status              VARCHAR(16) NOT NULL DEFAULT 'draft',
    current_version     INTEGER NOT NULL DEFAULT 0,
    created_by_user_id  UUID NOT NULL,
    updated_by_user_id  UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_operational_playbook_status CHECK (status IN ('draft', 'active', 'retired'))
);

CREATE INDEX idx_operational_playbook_tenant_status
    ON control_tower.operational_playbook (tenant_id, status);

CREATE TABLE IF NOT EXISTS control_tower.operational_playbook_version (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    playbook_id         UUID NOT NULL REFERENCES control_tower.operational_playbook(id) ON DELETE CASCADE,
    version             INTEGER NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'draft',
    created_by_user_id  UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_operational_playbook_version_status CHECK (status IN ('draft', 'active', 'retired')),
    CONSTRAINT uq_operational_playbook_version UNIQUE (tenant_id, playbook_id, version)
);

CREATE INDEX idx_operational_playbook_version_playbook
    ON control_tower.operational_playbook_version (playbook_id, version DESC);

CREATE TABLE IF NOT EXISTS control_tower.operational_playbook_step (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    playbook_version_id UUID NOT NULL REFERENCES control_tower.operational_playbook_version(id) ON DELETE CASCADE,
    sequence            INTEGER NOT NULL,
    title               VARCHAR(256) NOT NULL,
    description         TEXT,
    step_type           VARCHAR(32) NOT NULL DEFAULT 'instruction',
    required            BOOLEAN NOT NULL DEFAULT TRUE,
    estimated_duration_minutes INTEGER,
    action_code         VARCHAR(64),
    CONSTRAINT chk_operational_playbook_step_type CHECK (step_type IN ('instruction', 'checklist', 'operator_action', 'system_action')),
    CONSTRAINT uq_operational_playbook_step_sequence UNIQUE (playbook_version_id, sequence)
);

CREATE INDEX idx_operational_playbook_step_version
    ON control_tower.operational_playbook_step (playbook_version_id, sequence);

ALTER TABLE control_tower.automation_rule
    ADD CONSTRAINT fk_automation_rule_playbook
    FOREIGN KEY (playbook_id) REFERENCES control_tower.operational_playbook(id);

CREATE TABLE IF NOT EXISTS control_tower.automation_recommendation (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    rule_id             UUID NOT NULL REFERENCES control_tower.automation_rule(id),
    rule_version        INTEGER NOT NULL,
    playbook_id         UUID NOT NULL REFERENCES control_tower.operational_playbook(id),
    playbook_version    INTEGER NOT NULL,
    playbook_version_id UUID NOT NULL REFERENCES control_tower.operational_playbook_version(id),
    trigger_id          VARCHAR(128) NOT NULL,
    trigger_type        VARCHAR(64) NOT NULL,
    correlation_id      VARCHAR(128),
    causation_id        VARCHAR(128),
    shipment_id         UUID,
    work_item_type      VARCHAR(32),
    work_item_id        VARCHAR(64),
    case_id             UUID,
    risk_id             VARCHAR(128),
    exception_id        VARCHAR(128),
    status              VARCHAR(16) NOT NULL DEFAULT 'pending',
    match_explanation   JSONB NOT NULL DEFAULT '[]'::jsonb,
    idempotency_key     VARCHAR(256) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ,
    accepted_by_user_id UUID,
    accepted_at         TIMESTAMPTZ,
    dismissed_by_user_id UUID,
    dismissed_at        TIMESTAMPTZ,
    dismiss_reason      VARCHAR(32),
    completed_at        TIMESTAMPTZ,
    CONSTRAINT chk_automation_recommendation_status CHECK (status IN ('pending', 'accepted', 'dismissed', 'expired', 'completed')),
    CONSTRAINT chk_automation_recommendation_dismiss_reason CHECK (
        dismiss_reason IS NULL OR dismiss_reason IN ('not_relevant', 'already_handled', 'duplicate', 'false_positive', 'other')
    ),
    CONSTRAINT uq_automation_recommendation_idempotency UNIQUE (tenant_id, idempotency_key)
);

CREATE INDEX idx_automation_recommendation_tenant_status
    ON control_tower.automation_recommendation (tenant_id, status, created_at DESC);

CREATE INDEX idx_automation_recommendation_tenant_case
    ON control_tower.automation_recommendation (tenant_id, case_id)
    WHERE case_id IS NOT NULL;

CREATE INDEX idx_automation_recommendation_tenant_work_item
    ON control_tower.automation_recommendation (tenant_id, work_item_type, work_item_id)
    WHERE work_item_id IS NOT NULL;

CREATE INDEX idx_automation_recommendation_tenant_shipment
    ON control_tower.automation_recommendation (tenant_id, shipment_id)
    WHERE shipment_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS control_tower.playbook_execution (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    recommendation_id   UUID REFERENCES control_tower.automation_recommendation(id),
    playbook_id         UUID NOT NULL REFERENCES control_tower.operational_playbook(id),
    playbook_version    INTEGER NOT NULL,
    playbook_version_id UUID NOT NULL REFERENCES control_tower.operational_playbook_version(id),
    shipment_id         UUID,
    work_item_type      VARCHAR(32),
    work_item_id        VARCHAR(64),
    case_id             UUID,
    owner_user_id       UUID NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'not_started',
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_by_user_id  UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_playbook_execution_status CHECK (status IN ('not_started', 'in_progress', 'completed', 'cancelled')),
    CONSTRAINT uq_playbook_execution_recommendation UNIQUE (recommendation_id)
);

CREATE INDEX idx_playbook_execution_tenant_status
    ON control_tower.playbook_execution (tenant_id, status, updated_at DESC);

CREATE INDEX idx_playbook_execution_tenant_case
    ON control_tower.playbook_execution (tenant_id, case_id)
    WHERE case_id IS NOT NULL;

CREATE INDEX idx_playbook_execution_tenant_work_item
    ON control_tower.playbook_execution (tenant_id, work_item_type, work_item_id)
    WHERE work_item_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS control_tower.playbook_execution_step (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    execution_id            UUID NOT NULL REFERENCES control_tower.playbook_execution(id) ON DELETE CASCADE,
    playbook_step_id        UUID NOT NULL REFERENCES control_tower.operational_playbook_step(id),
    sequence                INTEGER NOT NULL,
    title                   VARCHAR(256) NOT NULL,
    description             TEXT,
    step_type               VARCHAR(32) NOT NULL,
    required                BOOLEAN NOT NULL DEFAULT TRUE,
    action_code             VARCHAR(64),
    status                  VARCHAR(16) NOT NULL DEFAULT 'pending',
    skip_reason             TEXT,
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    started_by_user_id      UUID,
    completed_by_user_id    UUID,
    CONSTRAINT chk_playbook_execution_step_status CHECK (status IN ('pending', 'in_progress', 'done', 'skipped')),
    CONSTRAINT uq_playbook_execution_step_sequence UNIQUE (execution_id, sequence)
);

CREATE INDEX idx_playbook_execution_step_execution
    ON control_tower.playbook_execution_step (execution_id, sequence);

CREATE TABLE IF NOT EXISTS control_tower.automation_audit_event (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    event_type          VARCHAR(64) NOT NULL,
    actor_type          VARCHAR(16) NOT NULL DEFAULT 'user',
    actor_user_id       UUID,
    rule_id             UUID,
    playbook_id         UUID,
    recommendation_id   UUID,
    execution_id        UUID,
    payload             JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_automation_audit_actor_type CHECK (actor_type IN ('user', 'system'))
);

CREATE INDEX idx_automation_audit_tenant_occurred
    ON control_tower.automation_audit_event (tenant_id, occurred_at DESC);

CREATE INDEX idx_automation_audit_tenant_rule
    ON control_tower.automation_audit_event (tenant_id, rule_id)
    WHERE rule_id IS NOT NULL;
