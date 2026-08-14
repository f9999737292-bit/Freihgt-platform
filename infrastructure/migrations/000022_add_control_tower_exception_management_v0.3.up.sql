ALTER TABLE control_tower.critical_event_workflow
    ADD COLUMN IF NOT EXISTS priority VARCHAR(8) NOT NULL DEFAULT 'p3',
    ADD COLUMN IF NOT EXISTS exception_category VARCHAR(64) NOT NULL DEFAULT 'other',
    ADD COLUMN IF NOT EXISTS business_impact VARCHAR(16) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS exception_activated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS acknowledge_due_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS assignment_due_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolution_due_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS escalation_level VARCHAR(16) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS ack_sla_breached_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS assign_sla_breached_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolve_sla_breached_at TIMESTAMPTZ;

ALTER TABLE control_tower.critical_event_workflow
    ADD CONSTRAINT chk_critical_event_workflow_priority
        CHECK (priority IN ('p1', 'p2', 'p3', 'p4'));

ALTER TABLE control_tower.critical_event_workflow
    ADD CONSTRAINT chk_critical_event_workflow_exception_category
        CHECK (exception_category IN (
            'delay', 'route_deviation', 'document_issue', 'vehicle_issue', 'driver_issue',
            'slot_issue', 'delivery_issue', 'pickup_issue', 'billing_issue', 'integration_issue',
            'data_quality', 'other'
        ));

ALTER TABLE control_tower.critical_event_workflow
    ADD CONSTRAINT chk_critical_event_workflow_business_impact
        CHECK (business_impact IN ('none', 'low', 'medium', 'high', 'critical'));

ALTER TABLE control_tower.critical_event_workflow
    ADD CONSTRAINT chk_critical_event_workflow_escalation_level
        CHECK (escalation_level IN ('none', 'level_1', 'level_2', 'level_3'));

UPDATE control_tower.critical_event_workflow
SET exception_activated_at = COALESCE(exception_activated_at, created_at)
WHERE exception_activated_at IS NULL;

ALTER TABLE control_tower.critical_event_workflow
    ALTER COLUMN exception_activated_at SET NOT NULL;

CREATE INDEX idx_critical_event_workflow_tenant_priority
    ON control_tower.critical_event_workflow (tenant_id, priority)
    WHERE status <> 'resolved';

CREATE INDEX idx_critical_event_workflow_tenant_resolution_due
    ON control_tower.critical_event_workflow (tenant_id, resolution_due_at)
    WHERE status <> 'resolved';

CREATE INDEX idx_critical_event_workflow_tenant_escalation
    ON control_tower.critical_event_workflow (tenant_id, escalation_level)
    WHERE status <> 'resolved';

ALTER TABLE control_tower.critical_event_action
    DROP CONSTRAINT IF EXISTS chk_critical_event_action_type;

ALTER TABLE control_tower.critical_event_action
    ADD CONSTRAINT chk_critical_event_action_type
        CHECK (action_type IN (
            'acknowledged', 'assigned', 'reassigned', 'resolved', 'reopened',
            'exception_updated', 'ack_sla_breached', 'assign_sla_breached', 'resolve_sla_breached',
            'escalation_changed'
        ));
