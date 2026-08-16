CREATE TABLE IF NOT EXISTS control_tower.shipment_risk (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    risk_key                VARCHAR(32) NOT NULL,
    shipment_id             UUID NOT NULL,
    predicted_exception_type VARCHAR(64) NOT NULL,
    score                   SMALLINT NOT NULL DEFAULT 0,
    risk_level              VARCHAR(16) NOT NULL DEFAULT 'none',
    status                  VARCHAR(16) NOT NULL DEFAULT 'active',
    first_detected_at       TIMESTAMPTZ NOT NULL,
    evaluated_at            TIMESTAMPTZ NOT NULL,
    next_evaluation_at      TIMESTAMPTZ,
    threatened_deadline_at  TIMESTAMPTZ,
    cleared_at              TIMESTAMPTZ,
    clear_reason            VARCHAR(64),
    materialized_at         TIMESTAMPTZ,
    actual_event_id         VARCHAR(32),
    mitigation_code         VARCHAR(64),
    mitigation_comment      TEXT,
    acknowledged_at         TIMESTAMPTZ,
    acknowledged_by_user_id UUID,
    mitigating_at           TIMESTAMPTZ,
    mitigating_by_user_id   UUID,
    version                 INTEGER NOT NULL DEFAULT 1,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_shipment_risk_identity UNIQUE (tenant_id, shipment_id, predicted_exception_type),
    CONSTRAINT uq_shipment_risk_key UNIQUE (tenant_id, risk_key),
    CONSTRAINT chk_shipment_risk_level CHECK (risk_level IN ('none', 'low', 'medium', 'high', 'critical')),
    CONSTRAINT chk_shipment_risk_status CHECK (status IN ('active', 'acknowledged', 'mitigating', 'cleared', 'materialized')),
    CONSTRAINT chk_shipment_risk_score CHECK (score >= 0 AND score <= 100)
);

CREATE TABLE IF NOT EXISTS control_tower.shipment_risk_assessment (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID NOT NULL,
    shipment_risk_id        UUID NOT NULL REFERENCES control_tower.shipment_risk(id) ON DELETE CASCADE,
    shipment_id             UUID NOT NULL,
    predicted_exception_type VARCHAR(64) NOT NULL,
    score                   SMALLINT NOT NULL,
    risk_level              VARCHAR(16) NOT NULL,
    status                  VARCHAR(16) NOT NULL,
    evaluated_at            TIMESTAMPTZ NOT NULL,
    signals_hash            VARCHAR(64) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS control_tower.shipment_risk_signal (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    assessment_id   UUID NOT NULL REFERENCES control_tower.shipment_risk_assessment(id) ON DELETE CASCADE,
    signal_code     VARCHAR(64) NOT NULL,
    severity        VARCHAR(16) NOT NULL,
    weight          SMALLINT NOT NULL DEFAULT 0,
    observed_at     TIMESTAMPTZ NOT NULL,
    source          VARCHAR(64) NOT NULL,
    value_json      JSONB,
    explanation_key VARCHAR(128) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS control_tower.shipment_risk_action (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    shipment_risk_id UUID NOT NULL REFERENCES control_tower.shipment_risk(id) ON DELETE CASCADE,
    action_type     VARCHAR(64) NOT NULL,
    actor_user_id   UUID,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata        JSONB,
    CONSTRAINT chk_shipment_risk_action_type CHECK (action_type IN (
        'risk_acknowledged', 'mitigation_started', 'mitigation_updated',
        'risk_cleared', 'risk_materialized'
    ))
);

CREATE INDEX idx_shipment_risk_tenant_active
    ON control_tower.shipment_risk (tenant_id, status)
    WHERE status IN ('active', 'acknowledged', 'mitigating');

CREATE INDEX idx_shipment_risk_tenant_shipment
    ON control_tower.shipment_risk (tenant_id, shipment_id);

CREATE INDEX idx_shipment_risk_tenant_level
    ON control_tower.shipment_risk (tenant_id, risk_level)
    WHERE status IN ('active', 'acknowledged', 'mitigating');

CREATE INDEX idx_shipment_risk_tenant_predicted_type
    ON control_tower.shipment_risk (tenant_id, predicted_exception_type)
    WHERE status IN ('active', 'acknowledged', 'mitigating');

CREATE INDEX idx_shipment_risk_tenant_evaluated
    ON control_tower.shipment_risk (tenant_id, evaluated_at DESC);

CREATE INDEX idx_shipment_risk_assessment_risk
    ON control_tower.shipment_risk_assessment (shipment_risk_id, evaluated_at DESC);
