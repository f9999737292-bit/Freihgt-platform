-- FREIGHT COST INTELLIGENCE v2.2E — tenant benchmark & explainable savings opportunities.

CREATE TABLE freight_cost.cost_analytics_benchmark_projection (
    tenant_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    cohort_type VARCHAR(32) NOT NULL,
    lane_key VARCHAR(256) NOT NULL,
    transport_mode VARCHAR(32) NOT NULL,
    equipment_type VARCHAR(32) NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    currency_code CHAR(3) NOT NULL,
    sample_count INT NOT NULL DEFAULT 0,
    mean_amount NUMERIC(18, 2),
    median_amount NUMERIC(18, 2),
    p25_amount NUMERIC(18, 2),
    p75_amount NUMERIC(18, 2),
    p90_amount NUMERIC(18, 2),
    min_amount NUMERIC(18, 2),
    max_amount NUMERIC(18, 2),
    data_quality VARCHAR(32) NOT NULL DEFAULT 'INSUFFICIENT_SAMPLE',
    rule_version INT NOT NULL DEFAULT 1,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_through TIMESTAMPTZ NOT NULL,
    projection_version INT NOT NULL,

    PRIMARY KEY (
        tenant_id, buyer_company_id, cohort_type, lane_key,
        transport_mode, equipment_type, period_start, period_grain, currency_code
    ),

    CONSTRAINT chk_benchmark_cohort_type CHECK (cohort_type IN ('LANE')),
    CONSTRAINT chk_benchmark_period_grain CHECK (period_grain IN ('MONTH')),
    CONSTRAINT chk_benchmark_currency CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_benchmark_sample_count CHECK (sample_count >= 0),
    CONSTRAINT chk_benchmark_data_quality CHECK (
        data_quality IN ('AVAILABLE', 'PARTIAL', 'INSUFFICIENT_SAMPLE', 'STALE', 'NOT_AVAILABLE')
    ),
    CONSTRAINT chk_benchmark_rule_version CHECK (rule_version > 0),
    CONSTRAINT chk_benchmark_projection_version CHECK (projection_version > 0)
);

CREATE INDEX idx_benchmark_tenant_period
    ON freight_cost.cost_analytics_benchmark_projection (
        tenant_id, buyer_company_id, period_start, currency_code
    );

CREATE INDEX idx_benchmark_tenant_lane
    ON freight_cost.cost_analytics_benchmark_projection (tenant_id, lane_key);

CREATE TABLE freight_cost.cost_analytics_opportunity_projection (
    tenant_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    opportunity_id UUID NOT NULL,
    opportunity_type VARCHAR(64) NOT NULL,
    scope VARCHAR(32) NOT NULL,
    entity_key VARCHAR(512) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    transport_order_id UUID,
    carrier_company_id UUID,
    lane_key VARCHAR(256),
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    observed_amount NUMERIC(18, 2) NOT NULL,
    baseline_amount NUMERIC(18, 2) NOT NULL,
    estimated_delta NUMERIC(18, 2) NOT NULL,
    sample_size INT NOT NULL DEFAULT 0,
    data_quality VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE',
    rule_version INT NOT NULL DEFAULT 1,
    evidence_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_through TIMESTAMPTZ NOT NULL,
    projection_version INT NOT NULL,

    PRIMARY KEY (tenant_id, opportunity_id),

    CONSTRAINT chk_opportunity_type CHECK (opportunity_type IN (
        'LANE_COST_OUTLIER',
        'COST_ABOVE_LANE_MEDIAN',
        'CARRIER_COST_OUTLIER',
        'HIGH_ACCESSORIAL_RATE',
        'REPEATED_VARIANCE'
    )),
    CONSTRAINT chk_opportunity_scope CHECK (scope IN ('LANE', 'CARRIER', 'ORDER', 'ACCESSORIAL', 'VARIANCE')),
    CONSTRAINT chk_opportunity_currency CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_opportunity_period_grain CHECK (period_grain IN ('MONTH')),
    CONSTRAINT chk_opportunity_sample_size CHECK (sample_size >= 0),
    CONSTRAINT chk_opportunity_delta_nonneg CHECK (estimated_delta >= 0),
    CONSTRAINT chk_opportunity_data_quality CHECK (
        data_quality IN ('AVAILABLE', 'PARTIAL', 'INSUFFICIENT_SAMPLE', 'STALE', 'NOT_AVAILABLE')
    ),
    CONSTRAINT chk_opportunity_rule_version CHECK (rule_version > 0),
    CONSTRAINT chk_opportunity_projection_version CHECK (projection_version > 0)
);

CREATE INDEX idx_opportunity_tenant_type
    ON freight_cost.cost_analytics_opportunity_projection (tenant_id, buyer_company_id, opportunity_type);

CREATE INDEX idx_opportunity_tenant_period
    ON freight_cost.cost_analytics_opportunity_projection (tenant_id, buyer_company_id, period_start, currency_code);

COMMENT ON TABLE freight_cost.cost_analytics_benchmark_projection IS
    'Derived v2.2E tenant-only lane cost benchmark statistics.';
COMMENT ON TABLE freight_cost.cost_analytics_opportunity_projection IS
    'Derived v2.2E explainable rule-based savings opportunities.';
