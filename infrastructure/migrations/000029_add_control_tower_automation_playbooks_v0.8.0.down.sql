DROP TABLE IF EXISTS control_tower.automation_audit_event;
DROP TABLE IF EXISTS control_tower.playbook_execution_step;
DROP TABLE IF EXISTS control_tower.playbook_execution;
DROP TABLE IF EXISTS control_tower.automation_recommendation;

ALTER TABLE IF EXISTS control_tower.automation_rule
    DROP CONSTRAINT IF EXISTS fk_automation_rule_playbook;

DROP TABLE IF EXISTS control_tower.operational_playbook_step;
DROP TABLE IF EXISTS control_tower.operational_playbook_version;
DROP TABLE IF EXISTS control_tower.operational_playbook;
DROP TABLE IF EXISTS control_tower.automation_rule_version;
DROP TABLE IF EXISTS control_tower.automation_rule;
