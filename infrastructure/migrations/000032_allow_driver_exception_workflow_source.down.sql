ALTER TABLE control_tower.critical_event_workflow
    DROP CONSTRAINT IF EXISTS chk_critical_event_workflow_source;

ALTER TABLE control_tower.critical_event_workflow
    ADD CONSTRAINT chk_critical_event_workflow_source
        CHECK (source = 'control-tower');
