ALTER TABLE control_tower.saved_view DROP CONSTRAINT IF EXISTS chk_saved_view_workspace_scope;
ALTER TABLE control_tower.saved_view DROP COLUMN IF EXISTS workspace_scope;

DROP TABLE IF EXISTS control_tower.operational_case_event;
DROP TABLE IF EXISTS control_tower.operational_case_decision;
DROP TABLE IF EXISTS control_tower.operational_case_action_item;
DROP TABLE IF EXISTS control_tower.operational_case_note_mention;
DROP TABLE IF EXISTS control_tower.operational_case_note;
DROP TABLE IF EXISTS control_tower.operational_case_participant;
DROP TABLE IF EXISTS control_tower.operational_case_active_work_link;
DROP TABLE IF EXISTS control_tower.operational_case_link;
DROP TABLE IF EXISTS control_tower.operational_case;
DROP TABLE IF EXISTS control_tower.operational_case_reference_counter;
