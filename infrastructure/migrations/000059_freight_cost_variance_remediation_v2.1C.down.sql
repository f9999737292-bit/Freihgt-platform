ALTER TABLE freight_cost.charge_code_mapping
    DROP CONSTRAINT IF EXISTS ex_charge_code_mapping_no_overlap;

DROP SEQUENCE IF EXISTS freight_cost.charge_code_mapping_version_seq;

ALTER TABLE freight_cost.cost_summary_projection
    DROP COLUMN IF EXISTS forecast_source_status,
    DROP COLUMN IF EXISTS derived_state_fingerprint;
