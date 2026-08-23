ALTER TABLE freight_cost.analytics_projection_coverage
    DROP COLUMN IF EXISTS missing_order_reference_count,
    DROP COLUMN IF EXISTS missing_carrier_display_count,
    DROP COLUMN IF EXISTS unmapped_charge_code_count,
    DROP COLUMN IF EXISTS excluded_cancelled_count,
    DROP COLUMN IF EXISTS excluded_rejected_count,
    DROP COLUMN IF EXISTS excluded_proposed_count;

DROP TABLE IF EXISTS freight_cost.cost_analytics_accessorial_period_projection;
DROP TABLE IF EXISTS freight_cost.cost_analytics_accessorial_fact;

ALTER TABLE freight_cost.cost_analytics_order_fact
    DROP COLUMN IF EXISTS lane_label,
    DROP COLUMN IF EXISTS carrier_display_name,
    DROP COLUMN IF EXISTS order_reference;
