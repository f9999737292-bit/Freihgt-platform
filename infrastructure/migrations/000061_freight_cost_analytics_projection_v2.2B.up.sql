-- FREIGHT COST INTELLIGENCE v2.2B — analytics projection core (derived read models only).
-- These tables are NOT authoritative financial sources; rebuild from cost_summary_projection.

CREATE TABLE freight_cost.cost_analytics_order_fact (
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    currency_code CHAR(3) NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    planned_amount NUMERIC(18, 2),
    accrued_amount NUMERIC(18, 2),
    current_actual_amount NUMERIC(18, 2),
    final_actual_amount NUMERIC(18, 2),
    current_variance_amount NUMERIC(18, 2),
    final_variance_amount NUMERIC(18, 2),
    data_stage VARCHAR(64) NOT NULL,
    financial_finality VARCHAR(32) NOT NULL,
    source_summary_revision BIGINT NOT NULL DEFAULT 0,
    source_summary_updated_at TIMESTAMPTZ NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, transport_order_id, currency_code),

    CONSTRAINT chk_cost_analytics_order_fact_period_grain
        CHECK (period_grain IN ('MONTH')),
    CONSTRAINT chk_cost_analytics_order_fact_currency
        CHECK (currency_code ~ '^[A-Z]{3}$')
);

CREATE INDEX idx_cost_analytics_order_fact_tenant_period
    ON freight_cost.cost_analytics_order_fact (tenant_id, buyer_company_id, period_start, period_grain, currency_code);

CREATE TABLE freight_cost.cost_analytics_period_projection (
    tenant_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    currency_code CHAR(3) NOT NULL,
    order_count INT NOT NULL DEFAULT 0,
    planned_total NUMERIC(18, 2),
    accrued_total NUMERIC(18, 2),
    current_actual_total NUMERIC(18, 2),
    final_actual_total NUMERIC(18, 2),
    current_variance_total NUMERIC(18, 2),
    final_variance_total NUMERIC(18, 2),
    reconciliation_open_count INT NOT NULL DEFAULT 0,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_through TIMESTAMPTZ NOT NULL,
    projection_version INT NOT NULL,

    PRIMARY KEY (tenant_id, buyer_company_id, period_start, period_grain, currency_code),

    CONSTRAINT chk_cost_analytics_period_order_count
        CHECK (order_count >= 0),
    CONSTRAINT chk_cost_analytics_period_reconciliation_count
        CHECK (reconciliation_open_count >= 0),
    CONSTRAINT chk_cost_analytics_period_projection_version
        CHECK (projection_version > 0),
    CONSTRAINT chk_cost_analytics_period_period_grain
        CHECK (period_grain IN ('MONTH'))
);

CREATE INDEX idx_cost_analytics_period_tenant_calculated
    ON freight_cost.cost_analytics_period_projection (tenant_id, calculated_at DESC);

CREATE TABLE freight_cost.analytics_projection_state (
    projection_name VARCHAR(64) NOT NULL,
    tenant_id UUID NOT NULL,
    projection_version INT NOT NULL,
    source_watermark TIMESTAMPTZ,
    last_successful_run_at TIMESTAMPTZ,
    calculated_at TIMESTAMPTZ,
    data_through TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL DEFAULT 'IDLE',
    last_error_code VARCHAR(64),
    last_error_message TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (projection_name, tenant_id),

    CONSTRAINT chk_analytics_projection_state_status
        CHECK (status IN ('IDLE', 'RUNNING', 'READY', 'STALE', 'ERROR')),
    CONSTRAINT chk_analytics_projection_state_version
        CHECK (projection_version > 0)
);

CREATE TABLE freight_cost.analytics_projection_dirty (
    tenant_id UUID NOT NULL,
    transport_order_id UUID NOT NULL,
    buyer_company_id UUID NOT NULL,
    currency_code CHAR(3) NOT NULL,
    period_start DATE NOT NULL,
    period_grain VARCHAR(16) NOT NULL DEFAULT 'MONTH',
    dirty_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_event_id UUID,

    PRIMARY KEY (tenant_id, transport_order_id, currency_code),

    CONSTRAINT chk_analytics_projection_dirty_period_grain
        CHECK (period_grain IN ('MONTH'))
);

CREATE INDEX idx_analytics_projection_dirty_poll
    ON freight_cost.analytics_projection_dirty (dirty_at ASC);

COMMENT ON TABLE freight_cost.cost_analytics_order_fact IS
    'Derived v2.2B order-level analytics facts. Rebuildable from cost_summary_projection.';
COMMENT ON TABLE freight_cost.cost_analytics_period_projection IS
    'Derived v2.2B period aggregates. Not authoritative financial source.';
COMMENT ON TABLE freight_cost.analytics_projection_state IS
    'Operational state for analytics projection rebuild/incremental processing.';
COMMENT ON TABLE freight_cost.analytics_projection_dirty IS
    'Transactional dirty queue for incremental analytics projection updates.';
