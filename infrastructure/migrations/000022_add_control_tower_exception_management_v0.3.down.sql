ALTER TABLE control_tower.critical_event_action
    DROP CONSTRAINT IF EXISTS chk_critical_event_action_type;

ALTER TABLE control_tower.critical_event_action
    ADD CONSTRAINT chk_critical_event_action_type
        CHECK (action_type IN ('acknowledged', 'assigned', 'reassigned', 'resolved', 'reopened'));

DROP INDEX IF EXISTS control_tower.idx_critical_event_workflow_tenant_escalation;
DROP INDEX IF EXISTS control_tower.idx_critical_event_workflow_tenant_resolution_due;
DROP INDEX IF EXISTS control_tower.idx_critical_event_workflow_tenant_priority;

ALTER TABLE control_tower.critical_event_workflow
    DROP CONSTRAINT IF EXISTS chk_critical_event_workflow_escalation_level,
    DROP CONSTRAINT IF EXISTS chk_critical_event_workflow_business_impact,
    DROP CONSTRAINT IF EXISTS chk_critical_event_workflow_exception_category,
    DROP CONSTRAINT IF EXISTS chk_critical_event_workflow_priority;

ALTER TABLE control_tower.critical_event_workflow
    DROP COLUMN IF EXISTS resolve_sla_breached_at,
    DROP COLUMN IF EXISTS assign_sla_breached_at,
    DROP COLUMN IF EXISTS ack_sla_breached_at,
    DROP COLUMN IF EXISTS escalation_level,
    DROP COLUMN IF EXISTS resolution_due_at,
    DROP COLUMN IF EXISTS assignment_due_at,
    DROP COLUMN IF EXISTS acknowledge_due_at,
    DROP COLUMN IF EXISTS exception_activated_at,
    DROP COLUMN IF EXISTS business_impact,
    DROP COLUMN IF EXISTS exception_category,
    DROP COLUMN IF EXISTS priority;
