ALTER TABLE freight_cost.cost_summary_projection
    ADD COLUMN IF NOT EXISTS attribution_mapping_evaluated_at TIMESTAMPTZ;
