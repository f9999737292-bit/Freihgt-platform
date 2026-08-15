DROP TABLE IF EXISTS control_tower.automation_tenant_action_policy;
DROP TABLE IF EXISTS control_tower.automation_timeout_escalation;
DROP TABLE IF EXISTS control_tower.automation_action_approval;
DROP TABLE IF EXISTS control_tower.automation_guarded_action;

ALTER TABLE control_tower.playbook_execution_step
    DROP CONSTRAINT IF EXISTS chk_playbook_execution_step_status;

ALTER TABLE control_tower.playbook_execution_step
    ADD CONSTRAINT chk_playbook_execution_step_status CHECK (
        status IN ('pending', 'in_progress', 'done', 'skipped')
    );
