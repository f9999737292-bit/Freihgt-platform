-- FREIGHT COST INTELLIGENCE v2.2C — lane & carrier period projections (derived read models).

ALTER TABLE freight_cost.cost_analytics_order_fact
    ADD COLUMN IF NOT EXISTS lane_key VARCHAR(256),
    ADD COLUMN IF NOT EXISTS origin_country CHAR(2),
    ADD COLUMN IF NOT EXISTS origin_city VARCHAR(128),
    ADD COLUMN IF NOT EXISTS destination_country CHAR(2),
    ADD COLUMN IF NOT EXISTS destination_city VARCHAR(128),
    ADD COLUMN IF NOT EXISTS transport_mode VARCHAR(32),
    ADD COLUMN IF NOT EXISTS equipment_type VARCHAR(32),
    ADD COLUMN IF NOT EXISTS lane_eligible BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_cost_analytics_order_fact_lane_period
    ON freight_cost.cost_analytics_order_fact (
        tenant_id, buyer_company_id, lane_key, transport_mode, equipment_type,
        period_start, period_grain, currency_code
    )
    WHERE lane_eligible = TRUE AND lane_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cost_analytics_order_fact_carrier_period
    ON freight_cost.cost_analytics_order_fact (
        tenant_id, buyer_company_id, carrier_company_id,
        period_start, period_grain, currency_code
    )
    WHERE carrier_company_id IS NOT NULL;

CREATE TABLE freight_cost.cost_analytics_lane_period_projection (
    tenant_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    lane_key VARCHAR(256) NOT NULL,
    transport_mode VARCHAR(32) NOT NULL,
    equipment_type VARCHAR(32) NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    currency_code CHAR(3) NOT NULL,
    order_count INT NOT NULL DEFAULT 0,
    carrier_count INT NOT NULL DEFAULT 0,
    planned_total NUMERIC(18, 2),
    accrued_total NUMERIC(18, 2),
    current_actual_total NUMERIC(18, 2),
    final_actual_total NUMERIC(18, 2),
    current_variance_total NUMERIC(18, 2),
    final_variance_total NUMERIC(18, 2),
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_through TIMESTAMPTZ NOT NULL,
    projection_version INT NOT NULL,

    PRIMARY KEY (
        tenant_id, buyer_company_id, lane_key, transport_mode, equipment_type,
        period_start, period_grain, currency_code
    ),

    CONSTRAINT chk_lane_period_order_count CHECK (order_count >= 0),
    CONSTRAINT chk_lane_period_carrier_count CHECK (carrier_count >= 0),
    CONSTRAINT chk_lane_period_projection_version CHECK (projection_version > 0),
    CONSTRAINT chk_lane_period_period_grain CHECK (period_grain IN ('MONTH')),
    CONSTRAINT chk_lane_period_currency CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_lane_period_tenant_calculated
    ON freight_cost.cost_analytics_lane_period_projection (tenant_id, calculated_at DESC);

CREATE TABLE freight_cost.cost_analytics_carrier_period_projection (
    tenant_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    currency_code CHAR(3) NOT NULL,
    order_count INT NOT NULL DEFAULT 0,
    lane_count INT NOT NULL DEFAULT 0,
    planned_total NUMERIC(18, 2),
    accrued_total NUMERIC(18, 2),
    current_actual_total NUMERIC(18, 2),
    final_actual_total NUMERIC(18, 2),
    current_variance_total NUMERIC(18, 2),
    final_variance_total NUMERIC(18, 2),
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_through TIMESTAMPTZ NOT NULL,
    projection_version INT NOT NULL,

    PRIMARY KEY (
        tenant_id, buyer_company_id, carrier_company_id,
        period_start, period_grain, currency_code
    ),

    CONSTRAINT chk_carrier_period_order_count CHECK (order_count >= 0),
    CONSTRAINT chk_carrier_period_lane_count CHECK (lane_count >= 0),
    CONSTRAINT chk_carrier_period_projection_version CHECK (projection_version > 0),
    CONSTRAINT chk_carrier_period_period_grain CHECK (period_grain IN ('MONTH')),
    CONSTRAINT chk_carrier_period_currency CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_carrier_period_tenant_calculated
    ON freight_cost.cost_analytics_carrier_period_projection (tenant_id, calculated_at DESC);

CREATE TABLE freight_cost.analytics_projection_coverage (
    projection_name VARCHAR(64) NOT NULL,
    tenant_id UUID NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL,
    source_order_count INT NOT NULL DEFAULT 0,
    eligible_order_count INT NOT NULL DEFAULT 0,
    excluded_order_count INT NOT NULL DEFAULT 0,
    excluded_missing_origin_city INT NOT NULL DEFAULT 0,
    excluded_missing_destination_city INT NOT NULL DEFAULT 0,
    excluded_missing_country INT NOT NULL DEFAULT 0,
    excluded_missing_mode INT NOT NULL DEFAULT 0,
    excluded_missing_carrier_id INT NOT NULL DEFAULT 0,
    data_quality VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE',

    PRIMARY KEY (projection_name, tenant_id),

    CONSTRAINT chk_analytics_projection_coverage_quality
        CHECK (data_quality IN ('AVAILABLE', 'PARTIAL', 'NOT_AVAILABLE'))
);

COMMENT ON TABLE freight_cost.cost_analytics_lane_period_projection IS
    'Derived v2.2C lane period aggregates. Not authoritative financial source.';
COMMENT ON TABLE freight_cost.cost_analytics_carrier_period_projection IS
    'Derived v2.2C carrier period aggregates. Not authoritative financial source.';
COMMENT ON TABLE freight_cost.analytics_projection_coverage IS
    'Coverage counters for analytics projection cohort eligibility.';
