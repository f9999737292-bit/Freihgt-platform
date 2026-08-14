CREATE TABLE IF NOT EXISTS control_tower.critical_event_workflow (
    tenant_id UUID NOT NULL,
    event_id VARCHAR(32) NOT NULL,
    shipment_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'control-tower',
    occurred_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    version INTEGER NOT NULL DEFAULT 1,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by_user_id UUID,
    assigned_to_user_id UUID,
    assigned_by_user_id UUID,
    assigned_at TIMESTAMPTZ,
    resolved_by_user_id UUID,
    resolved_at TIMESTAMPTZ,
    resolution_code VARCHAR(64),
    resolution_comment TEXT,
    last_reopened_at TIMESTAMPTZ,
    last_reopened_by_user_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, event_id),

    CONSTRAINT chk_critical_event_workflow_event_id_format
        CHECK (event_id ~ '^[0-9a-f]{32}$'),

    CONSTRAINT chk_critical_event_workflow_source
        CHECK (source = 'control-tower'),

    CONSTRAINT chk_critical_event_workflow_status
        CHECK (status IN ('open', 'acknowledged', 'assigned', 'resolved')),

    CONSTRAINT chk_critical_event_workflow_resolution_code
        CHECK (
            resolution_code IS NULL
            OR resolution_code IN ('issue_resolved', 'false_positive', 'duplicate', 'cancelled', 'other')
        )
);

CREATE INDEX idx_critical_event_workflow_tenant_status
    ON control_tower.critical_event_workflow (tenant_id, status);

CREATE INDEX idx_critical_event_workflow_tenant_shipment
    ON control_tower.critical_event_workflow (tenant_id, shipment_id);

CREATE TABLE IF NOT EXISTS control_tower.critical_event_action (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    event_id VARCHAR(32) NOT NULL,
    action_type VARCHAR(32) NOT NULL,
    actor_user_id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT chk_critical_event_action_event_id_format
        CHECK (event_id ~ '^[0-9a-f]{32}$'),

    CONSTRAINT chk_critical_event_action_type
        CHECK (action_type IN ('acknowledged', 'assigned', 'reassigned', 'resolved', 'reopened'))
);

CREATE INDEX idx_critical_event_action_tenant_event_occurred
    ON control_tower.critical_event_action (tenant_id, event_id, occurred_at ASC, id ASC);

-- Safe backfill from existing acknowledgement records (preserves v0.1 data).
INSERT INTO control_tower.critical_event_workflow (
    tenant_id,
    event_id,
    shipment_id,
    event_type,
    source,
    occurred_at,
    status,
    version,
    acknowledged_at,
    acknowledged_by_user_id,
    created_at,
    updated_at
)
SELECT
    tenant_id,
    event_id,
    shipment_id,
    event_type,
    source,
    occurred_at,
    'acknowledged',
    1,
    acknowledged_at,
    acknowledged_by_user_id,
    acknowledged_at,
    acknowledged_at
FROM control_tower.critical_event_acknowledgement
ON CONFLICT (tenant_id, event_id) DO NOTHING;

INSERT INTO control_tower.critical_event_action (
    tenant_id,
    event_id,
    action_type,
    actor_user_id,
    occurred_at,
    metadata
)
SELECT
    tenant_id,
    event_id,
    'acknowledged',
    acknowledged_by_user_id,
    acknowledged_at,
    '{}'::jsonb
FROM control_tower.critical_event_acknowledgement a
WHERE NOT EXISTS (
    SELECT 1
    FROM control_tower.critical_event_action act
    WHERE act.tenant_id = a.tenant_id
      AND act.event_id = a.event_id
      AND act.action_type = 'acknowledged'
      AND act.occurred_at = a.acknowledged_at
      AND act.actor_user_id = a.acknowledged_by_user_id
);
