-- Operational Case Management v0.6

CREATE TABLE IF NOT EXISTS control_tower.operational_case_reference_counter (
    tenant_id   UUID NOT NULL,
    year        SMALLINT NOT NULL,
    last_value  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, year)
);

CREATE TABLE IF NOT EXISTS control_tower.operational_case (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    reference           VARCHAR(32) NOT NULL,
    title               VARCHAR(256) NOT NULL,
    summary             TEXT,
    status              VARCHAR(32) NOT NULL DEFAULT 'open',
    derived_severity    VARCHAR(16) NOT NULL DEFAULT 'medium',
    effective_severity  VARCHAR(16) NOT NULL DEFAULT 'medium',
    severity_override   BOOLEAN NOT NULL DEFAULT FALSE,
    owner_user_id       UUID,
    created_by_user_id  UUID NOT NULL,
    resolution_code     VARCHAR(64),
    resolution_summary  TEXT,
    version             BIGINT NOT NULL DEFAULT 1,
    last_activity_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at         TIMESTAMPTZ,
    closed_at           TIMESTAMPTZ,
    CONSTRAINT chk_operational_case_status CHECK (status IN (
        'open', 'investigating', 'action_required', 'monitoring', 'resolved', 'closed'
    )),
    CONSTRAINT chk_operational_case_derived_severity CHECK (derived_severity IN ('critical', 'high', 'medium', 'low')),
    CONSTRAINT chk_operational_case_effective_severity CHECK (effective_severity IN ('critical', 'high', 'medium', 'low')),
    CONSTRAINT uq_operational_case_reference UNIQUE (tenant_id, reference)
);

CREATE INDEX idx_operational_case_tenant_status
    ON control_tower.operational_case (tenant_id, status);

CREATE INDEX idx_operational_case_tenant_owner
    ON control_tower.operational_case (tenant_id, owner_user_id);

CREATE INDEX idx_operational_case_tenant_severity
    ON control_tower.operational_case (tenant_id, effective_severity);

CREATE INDEX idx_operational_case_tenant_created
    ON control_tower.operational_case (tenant_id, created_at DESC);

CREATE INDEX idx_operational_case_tenant_last_activity
    ON control_tower.operational_case (tenant_id, last_activity_at DESC);

CREATE INDEX idx_operational_case_tenant_reference
    ON control_tower.operational_case (tenant_id, reference);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_link (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    case_id             UUID NOT NULL REFERENCES control_tower.operational_case(id) ON DELETE CASCADE,
    entity_type         VARCHAR(32) NOT NULL,
    entity_id           VARCHAR(64) NOT NULL,
    linked_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    linked_by_user_id   UUID NOT NULL,
    CONSTRAINT chk_operational_case_link_entity_type CHECK (entity_type IN (
        'shipment', 'transport_order', 'exception', 'risk', 'work_item'
    )),
    CONSTRAINT uq_operational_case_link UNIQUE (tenant_id, case_id, entity_type, entity_id)
);

CREATE INDEX idx_operational_case_link_tenant_case
    ON control_tower.operational_case_link (tenant_id, case_id);

CREATE INDEX idx_operational_case_link_tenant_entity
    ON control_tower.operational_case_link (tenant_id, entity_type, entity_id);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_active_work_link (
    tenant_id   UUID NOT NULL,
    entity_type VARCHAR(32) NOT NULL,
    entity_id   VARCHAR(64) NOT NULL,
    case_id     UUID NOT NULL REFERENCES control_tower.operational_case(id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, entity_type, entity_id)
);

CREATE INDEX idx_operational_case_active_work_link_case
    ON control_tower.operational_case_active_work_link (case_id);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_participant (
    case_id             UUID NOT NULL REFERENCES control_tower.operational_case(id) ON DELETE CASCADE,
    tenant_id           UUID NOT NULL,
    user_id             UUID NOT NULL,
    role                VARCHAR(16) NOT NULL,
    added_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by_user_id    UUID NOT NULL,
    PRIMARY KEY (case_id, user_id),
    CONSTRAINT chk_operational_case_participant_role CHECK (role IN ('owner', 'collaborator', 'observer'))
);

CREATE INDEX idx_operational_case_participant_tenant_user
    ON control_tower.operational_case_participant (tenant_id, user_id);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_note (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    case_id         UUID NOT NULL REFERENCES control_tower.operational_case(id) ON DELETE CASCADE,
    author_user_id  UUID NOT NULL,
    body            TEXT NOT NULL,
    visibility      VARCHAR(16) NOT NULL DEFAULT 'internal',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    edited_at       TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    CONSTRAINT chk_operational_case_note_visibility CHECK (visibility IN ('internal', 'customer_visible'))
);

CREATE INDEX idx_operational_case_note_case
    ON control_tower.operational_case_note (case_id, created_at DESC);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_note_mention (
    note_id             UUID NOT NULL REFERENCES control_tower.operational_case_note(id) ON DELETE CASCADE,
    mentioned_user_id   UUID NOT NULL,
    PRIMARY KEY (note_id, mentioned_user_id)
);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_action_item (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    case_id             UUID NOT NULL REFERENCES control_tower.operational_case(id) ON DELETE CASCADE,
    title               VARCHAR(256) NOT NULL,
    description         TEXT,
    status              VARCHAR(16) NOT NULL DEFAULT 'open',
    assignee_user_id    UUID,
    due_at              TIMESTAMPTZ,
    created_by_user_id  UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    CONSTRAINT chk_operational_case_action_item_status CHECK (status IN (
        'open', 'in_progress', 'done', 'cancelled'
    ))
);

CREATE INDEX idx_operational_case_action_item_case
    ON control_tower.operational_case_action_item (case_id, status);

CREATE INDEX idx_operational_case_action_item_assignee
    ON control_tower.operational_case_action_item (tenant_id, assignee_user_id);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_decision (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    case_id             UUID NOT NULL REFERENCES control_tower.operational_case(id) ON DELETE CASCADE,
    decision            TEXT NOT NULL,
    rationale           TEXT,
    decided_by_user_id  UUID NOT NULL,
    decided_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_operational_case_decision_case
    ON control_tower.operational_case_decision (case_id, decided_at DESC);

CREATE TABLE IF NOT EXISTS control_tower.operational_case_event (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    case_id         UUID NOT NULL REFERENCES control_tower.operational_case(id) ON DELETE CASCADE,
    source          VARCHAR(16) NOT NULL,
    action_type     VARCHAR(64) NOT NULL,
    actor_user_id   UUID,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_operational_case_event_case
    ON control_tower.operational_case_event (case_id, occurred_at DESC);

CREATE INDEX idx_operational_case_event_tenant
    ON control_tower.operational_case_event (tenant_id, occurred_at DESC);

-- Extend saved views for cases workspace scope (backwards compatible)
ALTER TABLE control_tower.saved_view
    ADD COLUMN IF NOT EXISTS workspace_scope VARCHAR(16) NOT NULL DEFAULT 'work_items';

ALTER TABLE control_tower.saved_view
    DROP CONSTRAINT IF EXISTS chk_saved_view_workspace_scope;

ALTER TABLE control_tower.saved_view
    ADD CONSTRAINT chk_saved_view_workspace_scope CHECK (workspace_scope IN ('work_items', 'cases'));
