-- Risk operational ownership (separate from lifecycle status)
ALTER TABLE control_tower.shipment_risk
    ADD COLUMN IF NOT EXISTS owner_user_id UUID,
    ADD COLUMN IF NOT EXISTS owned_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS owned_by_user_id UUID;

CREATE INDEX IF NOT EXISTS idx_shipment_risk_tenant_owner_active
    ON control_tower.shipment_risk (tenant_id, owner_user_id)
    WHERE status IN ('active', 'acknowledged', 'mitigating') AND owner_user_id IS NOT NULL;

ALTER TABLE control_tower.shipment_risk_action
    DROP CONSTRAINT IF EXISTS chk_shipment_risk_action_type;

ALTER TABLE control_tower.shipment_risk_action
    ADD CONSTRAINT chk_shipment_risk_action_type CHECK (action_type IN (
        'risk_acknowledged', 'mitigation_started', 'mitigation_updated',
        'risk_cleared', 'risk_materialized',
        'risk_claimed', 'risk_assigned', 'risk_reassigned', 'risk_unassigned'
    ));

ALTER TABLE control_tower.critical_event_action
    DROP CONSTRAINT IF EXISTS chk_critical_event_action_type;

ALTER TABLE control_tower.critical_event_action
    ADD CONSTRAINT chk_critical_event_action_type CHECK (action_type IN (
        'acknowledged', 'assigned', 'reassigned', 'resolved', 'reopened',
        'exception_updated', 'ack_sla_breached', 'assign_sla_breached',
        'resolve_sla_breached', 'escalation_changed',
        'claimed', 'unassigned'
    ));

CREATE TABLE IF NOT EXISTS control_tower.saved_view (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    owner_user_id   UUID NOT NULL,
    name            VARCHAR(128) NOT NULL,
    scope           VARCHAR(16) NOT NULL DEFAULT 'private',
    filter_schema_version SMALLINT NOT NULL DEFAULT 1,
    filters         JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort            JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_saved_view_scope CHECK (scope IN ('private', 'shared')),
    CONSTRAINT uq_saved_view_name UNIQUE (tenant_id, owner_user_id, name)
);

CREATE INDEX idx_saved_view_tenant_owner
    ON control_tower.saved_view (tenant_id, owner_user_id);

CREATE INDEX idx_saved_view_tenant_scope
    ON control_tower.saved_view (tenant_id, scope);

CREATE TABLE IF NOT EXISTS control_tower.user_workspace_preference (
    tenant_id       UUID NOT NULL,
    user_id         UUID NOT NULL,
    default_view_id UUID REFERENCES control_tower.saved_view(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE IF NOT EXISTS control_tower.shift_handoff (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    from_user_id    UUID NOT NULL,
    to_user_id      UUID,
    title           VARCHAR(256),
    note            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shift_handoff_tenant_created
    ON control_tower.shift_handoff (tenant_id, created_at DESC);

CREATE INDEX idx_shift_handoff_tenant_from
    ON control_tower.shift_handoff (tenant_id, from_user_id);

CREATE INDEX idx_shift_handoff_tenant_to
    ON control_tower.shift_handoff (tenant_id, to_user_id);

CREATE TABLE IF NOT EXISTS control_tower.shift_handoff_item (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    handoff_id      UUID NOT NULL REFERENCES control_tower.shift_handoff(id) ON DELETE CASCADE,
    item_type       VARCHAR(16) NOT NULL,
    source_id       VARCHAR(64) NOT NULL,
    shipment_id     UUID,
    outcome         VARCHAR(16) NOT NULL DEFAULT 'transferred',
    error_code      VARCHAR(64),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_handoff_item_type CHECK (item_type IN ('exception', 'risk')),
    CONSTRAINT chk_handoff_item_outcome CHECK (outcome IN ('transferred', 'failed'))
);

CREATE INDEX idx_shift_handoff_item_handoff
    ON control_tower.shift_handoff_item (handoff_id);
