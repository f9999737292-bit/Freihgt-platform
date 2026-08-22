ALTER TABLE freight_cost.cost_summary_projection
    DROP COLUMN IF EXISTS projection_revision,
    DROP COLUMN IF EXISTS attribution_mapping_version,
    DROP COLUMN IF EXISTS forecast_exposure,
    DROP COLUMN IF EXISTS final_variance_percent,
    DROP COLUMN IF EXISTS current_variance_percent,
    DROP COLUMN IF EXISTS final_variance_amount,
    DROP COLUMN IF EXISTS current_variance_amount;
