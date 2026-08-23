-- FREIGHT COST INTELLIGENCE v2.2D — accessorial analytics & dimension enrichment.

ALTER TABLE freight_cost.cost_analytics_order_fact
    ADD COLUMN IF NOT EXISTS order_reference VARCHAR(128),
    ADD COLUMN IF NOT EXISTS carrier_display_name VARCHAR(256),
    ADD COLUMN IF NOT EXISTS lane_label VARCHAR(512);

CREATE TABLE freight_cost.cost_analytics_accessorial_fact (
    tenant_id UUID NOT NULL,
    accessorial_id UUID NOT NULL,
    currency_code CHAR(3) NOT NULL,
    transport_order_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    settlement_id UUID NOT NULL,
    charge_code VARCHAR(50) NOT NULL,
    normalized_category VARCHAR(50) NOT NULL,
    amount NUMERIC(18, 2) NOT NULL,
    status VARCHAR(50) NOT NULL,
    mapping_version BIGINT NOT NULL,
    mapping_evaluated_at TIMESTAMPTZ NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    eligible BOOLEAN NOT NULL DEFAULT FALSE,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, accessorial_id, currency_code),

    CONSTRAINT chk_accessorial_fact_currency CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_accessorial_fact_period_grain CHECK (period_grain IN ('MONTH')),
    CONSTRAINT chk_accessorial_fact_category CHECK (
        normalized_category IN ('DETENTION', 'FUEL', 'WAITING', 'LUMPER', 'ACCESSORIAL', 'OTHER')
    )
);

CREATE INDEX idx_accessorial_fact_tenant_period
    ON freight_cost.cost_analytics_accessorial_fact (
        tenant_id, buyer_company_id, normalized_category, period_start, period_grain, currency_code
    )
    WHERE eligible = TRUE;

CREATE INDEX idx_accessorial_fact_tenant_order
    ON freight_cost.cost_analytics_accessorial_fact (tenant_id, transport_order_id);

CREATE TABLE freight_cost.cost_analytics_accessorial_period_projection (
    tenant_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    normalized_category VARCHAR(50) NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    currency_code CHAR(3) NOT NULL,
    total_amount NUMERIC(18, 2),
    order_count INT NOT NULL DEFAULT 0,
    line_count INT NOT NULL DEFAULT 0,
    share_of_spend NUMERIC(18, 6),
    accessorial_order_rate NUMERIC(18, 6),
    freight_spend_total NUMERIC(18, 2),
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_through TIMESTAMPTZ NOT NULL,
    projection_version INT NOT NULL,

    PRIMARY KEY (
        tenant_id, buyer_company_id, normalized_category,
        period_start, period_grain, currency_code
    ),

    CONSTRAINT chk_accessorial_period_order_count CHECK (order_count >= 0),
    CONSTRAINT chk_accessorial_period_line_count CHECK (line_count >= 0),
    CONSTRAINT chk_accessorial_period_projection_version CHECK (projection_version > 0),
    CONSTRAINT chk_accessorial_period_period_grain CHECK (period_grain IN ('MONTH')),
    CONSTRAINT chk_accessorial_period_currency CHECK (currency_code ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_accessorial_period_category CHECK (
        normalized_category IN ('DETENTION', 'FUEL', 'WAITING', 'LUMPER', 'ACCESSORIAL', 'OTHER')
    )
);

CREATE INDEX idx_accessorial_period_tenant_calculated
    ON freight_cost.cost_analytics_accessorial_period_projection (tenant_id, calculated_at DESC);

ALTER TABLE freight_cost.analytics_projection_coverage
    ADD COLUMN IF NOT EXISTS excluded_proposed_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS excluded_rejected_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS excluded_cancelled_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS unmapped_charge_code_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS missing_carrier_display_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS missing_order_reference_count INT NOT NULL DEFAULT 0;

COMMENT ON TABLE freight_cost.cost_analytics_accessorial_fact IS
    'Derived v2.2D accessorial line facts with pinned charge classification.';
COMMENT ON TABLE freight_cost.cost_analytics_accessorial_period_projection IS
    'Derived v2.2D accessorial period aggregates by normalized category.';
