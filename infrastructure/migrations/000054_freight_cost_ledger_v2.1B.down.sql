DROP TABLE IF EXISTS freight_cost.cost_summary_projection;
DROP TABLE IF EXISTS freight_cost.source_cursor;
DROP TRIGGER IF EXISTS trg_cost_entry_deny_delete ON freight_cost.cost_entry;
DROP TRIGGER IF EXISTS trg_cost_entry_deny_update ON freight_cost.cost_entry;
DROP FUNCTION IF EXISTS freight_cost.deny_cost_entry_mutation();
DROP TABLE IF EXISTS freight_cost.cost_entry;
DROP SCHEMA IF EXISTS freight_cost;
