ALTER TABLE freight_cost.cost_summary_projection
    ADD COLUMN IF NOT EXISTS current_variance_amount NUMERIC(18, 2),
    ADD COLUMN IF NOT EXISTS final_variance_amount NUMERIC(18, 2),
    ADD COLUMN IF NOT EXISTS current_variance_percent NUMERIC(9, 4),
    ADD COLUMN IF NOT EXISTS final_variance_percent NUMERIC(9, 4),
    ADD COLUMN IF NOT EXISTS forecast_exposure NUMERIC(18, 2),
    ADD COLUMN IF NOT EXISTS attribution_mapping_version BIGINT,
    ADD COLUMN IF NOT EXISTS projection_revision BIGINT NOT NULL DEFAULT 1;
