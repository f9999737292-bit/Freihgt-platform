-- Control Tower Guarded Automatic Actions v0.8.1

ALTER TABLE control_tower.playbook_execution_step
    DROP CONSTRAINT IF EXISTS chk_playbook_execution_step_status;

ALTER TABLE control_tower.playbook_execution_step
    ADD CONSTRAINT chk_playbook_execution_step_status CHECK (
        status IN (
            'pending', 'in_progress', 'done', 'skipped',
            'waiting_approval', 'waiting_response', 'denied', 'failed', 'rejected', 'timed_out'
        )
    );

CREATE TABLE IF NOT EXISTS control_tower.automation_guarded_action (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    execution_id        UUID NOT NULL REFERENCES control_tower.playbook_execution(id) ON DELETE CASCADE,
    execution_step_id   UUID NOT NULL REFERENCES control_tower.playbook_execution_step(id) ON DELETE CASCADE,
    action_type         VARCHAR(64) NOT NULL,
    safety_class        VARCHAR(32) NOT NULL,
    guard_decision      VARCHAR(32) NOT NULL,
    guard_reason        TEXT,
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    driver_id           UUID,
    shipment_id         UUID,
    driver_task_id      UUID,
    correlation_id      VARCHAR(128),
    source_event_id     VARCHAR(128),
    idempotency_key     VARCHAR(256) NOT NULL,
    response_payload    JSONB,
    error_reason        TEXT,
    expires_at          TIMESTAMPTZ,
    dispatched_at       TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version             INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_automation_guarded_action_idempotency UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT uq_automation_guarded_action_step UNIQUE (tenant_id, execution_id, execution_step_id),
    CONSTRAINT chk_automation_guarded_action_status CHECK (
        status IN (
            'pending', 'waiting_approval', 'running', 'waiting_response',
            'succeeded', 'failed', 'denied', 'rejected', 'timed_out', 'skipped'
        )
    ),
    CONSTRAINT chk_automation_guarded_action_guard_decision CHECK (
        guard_decision IN ('allow', 'require_approval', 'deny', 'skip')
    )
);

CREATE INDEX idx_automation_guarded_action_execution
    ON control_tower.automation_guarded_action (tenant_id, execution_id);

CREATE INDEX idx_automation_guarded_action_driver_task
    ON control_tower.automation_guarded_action (tenant_id, driver_task_id)
    WHERE driver_task_id IS NOT NULL;

CREATE INDEX idx_automation_guarded_action_status
    ON control_tower.automation_guarded_action (tenant_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS control_tower.automation_action_approval (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    guarded_action_id   UUID NOT NULL REFERENCES control_tower.automation_guarded_action(id) ON DELETE CASCADE,
    required_level      VARCHAR(16) NOT NULL DEFAULT 'none',
    status              VARCHAR(16) NOT NULL DEFAULT 'pending',
    requested_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at         TIMESTAMPTZ,
    approved_by         UUID,
    rejected_at         TIMESTAMPTZ,
    rejected_by         UUID,
    reason              TEXT,
    version             INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_automation_action_approval_action UNIQUE (guarded_action_id),
    CONSTRAINT chk_automation_action_approval_status CHECK (
        status IN ('pending', 'approved', 'rejected')
    ),
    CONSTRAINT chk_automation_action_approval_level CHECK (
        required_level IN ('none', 'operator', 'supervisor')
    )
);

CREATE INDEX idx_automation_action_approval_tenant_status
    ON control_tower.automation_action_approval (tenant_id, status, requested_at DESC);

CREATE TABLE IF NOT EXISTS control_tower.automation_timeout_escalation (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    guarded_action_id   UUID NOT NULL REFERENCES control_tower.automation_guarded_action(id) ON DELETE CASCADE,
    case_id             UUID,
    idempotency_key     VARCHAR(256) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_automation_timeout_escalation_idempotency UNIQUE (tenant_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS control_tower.automation_tenant_action_policy (
    tenant_id           UUID NOT NULL,
    action_type         VARCHAR(64) NOT NULL,
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    approval_level      VARCHAR(16),
    priority_ceiling    VARCHAR(16),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, action_type),
    CONSTRAINT chk_automation_tenant_action_policy_approval CHECK (
        approval_level IS NULL OR approval_level IN ('none', 'operator', 'supervisor')
    ),
    CONSTRAINT chk_automation_tenant_action_policy_priority CHECK (
        priority_ceiling IS NULL OR priority_ceiling IN ('NORMAL', 'HIGH', 'CRITICAL')
    )
);
