DROP TABLE IF EXISTS freight_cost.analytics_projection_coverage;
DROP TABLE IF EXISTS freight_cost.cost_analytics_carrier_period_projection;
DROP TABLE IF EXISTS freight_cost.cost_analytics_lane_period_projection;

DROP INDEX IF EXISTS freight_cost.idx_cost_analytics_order_fact_carrier_period;
DROP INDEX IF EXISTS freight_cost.idx_cost_analytics_order_fact_lane_period;

ALTER TABLE freight_cost.cost_analytics_order_fact
    DROP COLUMN IF EXISTS lane_eligible,
    DROP COLUMN IF EXISTS equipment_type,
    DROP COLUMN IF EXISTS transport_mode,
    DROP COLUMN IF EXISTS destination_city,
    DROP COLUMN IF EXISTS destination_country,
    DROP COLUMN IF EXISTS origin_city,
    DROP COLUMN IF EXISTS origin_country,
    DROP COLUMN IF EXISTS lane_key;
