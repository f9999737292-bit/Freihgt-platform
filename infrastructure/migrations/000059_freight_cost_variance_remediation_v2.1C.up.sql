ALTER TABLE freight_cost.cost_summary_projection
    ADD COLUMN IF NOT EXISTS forecast_source_status VARCHAR(16),
    ADD COLUMN IF NOT EXISTS derived_state_fingerprint VARCHAR(64);

CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE freight_cost.charge_code_mapping
    DROP CONSTRAINT IF EXISTS ex_charge_code_mapping_no_overlap;

ALTER TABLE freight_cost.charge_code_mapping
    ADD CONSTRAINT ex_charge_code_mapping_no_overlap
    EXCLUDE USING gist (
        mapping_scope WITH =,
        (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid)) WITH =,
        source_charge_code_normalized WITH =,
        tstzrange(effective_from, COALESCE(effective_to, 'infinity'::timestamptz), '[)') WITH &&
    );

CREATE SEQUENCE IF NOT EXISTS freight_cost.charge_code_mapping_version_seq START WITH 2;
