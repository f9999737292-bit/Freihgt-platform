-- RFx / Tender Platform v0.3 — scoring, allocation, quota balance, award foundation

CREATE TABLE rfx.scoring_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    code VARCHAR(100) NOT NULL,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_scoring_template_code UNIQUE (tenant_id, code),
    CONSTRAINT chk_scoring_template_status CHECK (status IN ('ACTIVE', 'ARCHIVED'))
);

CREATE INDEX idx_scoring_templates_tenant ON rfx.scoring_templates(tenant_id);

CREATE TABLE rfx.scoring_template_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    scoring_template_id UUID NOT NULL REFERENCES rfx.scoring_templates(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    factors JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT uq_scoring_template_version UNIQUE (scoring_template_id, version_number)
);

CREATE INDEX idx_scoring_template_versions_tenant ON rfx.scoring_template_versions(tenant_id);

CREATE TABLE rfx.tender_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    scoring_template_version_id UUID REFERENCES rfx.scoring_template_versions(id),
    status VARCHAR(50) NOT NULL DEFAULT 'IN_PROGRESS',
    scoring_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    qualification_rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    frozen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_tender_evaluation_status CHECK (
        status IN ('IN_PROGRESS', 'COMPLETED', 'FROZEN')
    )
);

CREATE INDEX idx_tender_evaluations_event ON rfx.tender_evaluations(rfx_event_id);
CREATE INDEX idx_tender_evaluations_tenant ON rfx.tender_evaluations(tenant_id);

CREATE TABLE rfx.tender_qualification_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    evaluation_id UUID NOT NULL REFERENCES rfx.tender_evaluations(id) ON DELETE CASCADE,
    rfx_lot_id UUID REFERENCES rfx.rfx_lots(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    result VARCHAR(20) NOT NULL,
    reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tender_qualification UNIQUE (evaluation_id, rfx_lot_id, carrier_company_id),
    CONSTRAINT chk_tender_qualification_result CHECK (result IN ('QUALIFIED', 'DISQUALIFIED'))
);

CREATE INDEX idx_tender_qualification_evaluation ON rfx.tender_qualification_results(evaluation_id);

CREATE TABLE rfx.tender_carrier_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    evaluation_id UUID NOT NULL REFERENCES rfx.tender_evaluations(id) ON DELETE CASCADE,
    rfx_lot_id UUID REFERENCES rfx.rfx_lots(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    bid_revision_id UUID,
    total_score NUMERIC(10,4) NOT NULL,
    price_score NUMERIC(10,4) NOT NULL DEFAULT 0,
    sla_score NUMERIC(10,4) NOT NULL DEFAULT 0,
    carrier_kpi_score NUMERIC(10,4) NOT NULL DEFAULT 0,
    capacity_score NUMERIC(10,4) NOT NULL DEFAULT 0,
    reliability_score NUMERIC(10,4) NOT NULL DEFAULT 0,
    transit_time_score NUMERIC(10,4) NOT NULL DEFAULT 0,
    breakdown JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tender_carrier_score UNIQUE (evaluation_id, rfx_lot_id, carrier_company_id)
);

CREATE INDEX idx_tender_carrier_scores_evaluation ON rfx.tender_carrier_scores(evaluation_id);

CREATE TABLE rfx.allocation_scenarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    evaluation_id UUID NOT NULL REFERENCES rfx.tender_evaluations(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    strategy VARCHAR(50) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT chk_allocation_strategy CHECK (
        strategy IN (
            'WINNER_TAKES_MOST', 'DUAL_SOURCE', 'DIVERSIFIED', 'EQUAL_SPLIT',
            'SCORE_WEIGHTED', 'CAPACITY_WEIGHTED', 'MANUAL'
        )
    ),
    CONSTRAINT chk_allocation_scenario_status CHECK (
        status IN ('DRAFT', 'COMPUTED', 'INFEASIBLE', 'APPLIED')
    )
);

CREATE INDEX idx_allocation_scenarios_evaluation ON rfx.allocation_scenarios(evaluation_id);

CREATE TABLE rfx.allocation_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    scenario_id UUID NOT NULL REFERENCES rfx.allocation_scenarios(id) ON DELETE CASCADE,
    rfx_lot_id UUID REFERENCES rfx.rfx_lots(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    score NUMERIC(10,4) NOT NULL DEFAULT 0,
    target_share_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    base_share_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    balance_adjustment_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    proposed_share_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    committed_capacity NUMERIC(18,3) NOT NULL DEFAULT 0,
    proposed_volume NUMERIC(18,3) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_allocation_result UNIQUE (scenario_id, rfx_lot_id, carrier_company_id)
);

CREATE INDEX idx_allocation_results_scenario ON rfx.allocation_results(scenario_id);

CREATE TABLE rfx.quota_balance_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    rfx_lot_id UUID REFERENCES rfx.rfx_lots(id) ON DELETE CASCADE,
    period_type VARCHAR(50) NOT NULL DEFAULT 'CONTRACT_PERIOD',
    tolerance_pct NUMERIC(5,2) NOT NULL DEFAULT 5.00,
    carry_balance BOOLEAN NOT NULL DEFAULT true,
    max_correction_pct NUMERIC(5,2) NOT NULL DEFAULT 10.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT chk_quota_period_type CHECK (
        period_type IN ('MONTH', 'QUARTER', 'CONTRACT_PERIOD')
    )
);

CREATE INDEX idx_quota_balance_policies_event ON rfx.quota_balance_policies(rfx_event_id);

CREATE TABLE rfx.quota_balance_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    policy_id UUID NOT NULL REFERENCES rfx.quota_balance_policies(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    target_share_pct NUMERIC(8,4) NOT NULL,
    CONSTRAINT uq_quota_balance_target UNIQUE (policy_id, carrier_company_id)
);

CREATE TABLE rfx.quota_balance_positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    policy_id UUID NOT NULL REFERENCES rfx.quota_balance_policies(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    target_share_pct NUMERIC(8,4) NOT NULL,
    actual_share_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    balance_pp NUMERIC(8,4) NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_quota_balance_position UNIQUE (policy_id, carrier_company_id),
    CONSTRAINT chk_quota_balance_status CHECK (
        status IN ('UNDERALLOCATED', 'BALANCED', 'OVERALLOCATED')
    )
);

CREATE TABLE rfx.quota_ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    policy_id UUID NOT NULL REFERENCES rfx.quota_balance_policies(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    period_key VARCHAR(50) NOT NULL,
    target_share_pct NUMERIC(8,4) NOT NULL,
    actual_share_pct NUMERIC(8,4) NOT NULL,
    adjustment_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    source_type VARCHAR(50) NOT NULL,
    source_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_quota_ledger_policy ON rfx.quota_ledger_entries(policy_id);

CREATE TABLE rfx.award_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id) ON DELETE CASCADE,
    evaluation_id UUID REFERENCES rfx.tender_evaluations(id),
    allocation_scenario_id UUID REFERENCES rfx.allocation_scenarios(id),
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT_PROPOSAL',
    scoring_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    submitted_at TIMESTAMPTZ,
    approved_at TIMESTAMPTZ,
    approved_by UUID,
    rejected_at TIMESTAMPTZ,
    rejected_by UUID,
    idempotency_key VARCHAR(200),
    CONSTRAINT uq_award_proposal_idempotency UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT chk_award_proposal_status CHECK (
        status IN ('DRAFT_PROPOSAL', 'PENDING_APPROVAL', 'APPROVED', 'REJECTED', 'AWARDED')
    )
);

CREATE INDEX idx_award_proposals_event ON rfx.award_proposals(rfx_event_id);

CREATE TABLE rfx.award_proposal_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    award_proposal_id UUID NOT NULL REFERENCES rfx.award_proposals(id) ON DELETE CASCADE,
    rfx_lot_id UUID REFERENCES rfx.rfx_lots(id) ON DELETE CASCADE,
    carrier_company_id UUID NOT NULL,
    share_pct NUMERIC(8,4) NOT NULL,
    volume NUMERIC(18,3) NOT NULL DEFAULT 0,
    expected_cost NUMERIC(18,2) NOT NULL DEFAULT 0,
    currency_code CHAR(3) DEFAULT 'RUB',
    score NUMERIC(10,4) NOT NULL DEFAULT 0,
    base_share_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    balance_adjustment_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
    CONSTRAINT uq_award_proposal_line UNIQUE (award_proposal_id, rfx_lot_id, carrier_company_id)
);

CREATE TABLE rfx.awards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    award_proposal_id UUID NOT NULL UNIQUE REFERENCES rfx.award_proposals(id),
    rfx_event_id UUID NOT NULL REFERENCES rfx.rfx_events(id),
    status VARCHAR(50) NOT NULL DEFAULT 'FINALIZED',
    finalized_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finalized_by UUID,
    idempotency_key VARCHAR(200),
    CONSTRAINT uq_award_idempotency UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT chk_award_status CHECK (status IN ('FINALIZED', 'CANCELLED'))
);

CREATE TABLE rfx.award_transport_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    award_id UUID NOT NULL REFERENCES rfx.awards(id) ON DELETE CASCADE,
    transport_order_id UUID NOT NULL,
    carrier_company_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    idempotency_key VARCHAR(200),
    CONSTRAINT uq_award_transport_order UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT uq_award_transport_order_pair UNIQUE (award_id, transport_order_id)
);

-- Bid revisions (freight request + enterprise response versioning)
CREATE TABLE rfx.bid_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    bid_id UUID NOT NULL REFERENCES rfx.bids(id) ON DELETE CASCADE,
    revision_number INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    total_amount NUMERIC(18,2),
    currency_code CHAR(3) DEFAULT 'RUB',
    capacity_units NUMERIC(18,3),
    transit_hours NUMERIC(10,2),
    sla_score_input NUMERIC(5,2),
    carrier_kpi_score_input NUMERIC(5,2),
    reliability_score_input NUMERIC(5,2),
    comment TEXT,
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    idempotency_key VARCHAR(200),
    CONSTRAINT uq_bid_revision UNIQUE (bid_id, revision_number),
    CONSTRAINT uq_bid_revision_idempotency UNIQUE (tenant_id, idempotency_key)
);

CREATE UNIQUE INDEX uq_bid_active_revision ON rfx.bid_revisions(bid_id) WHERE is_active = true;

-- Extend rfx_responses with commercial bid fields for enterprise evaluation
ALTER TABLE rfx.rfx_responses
    ADD COLUMN IF NOT EXISTS price_amount NUMERIC(18,2),
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) DEFAULT 'RUB',
    ADD COLUMN IF NOT EXISTS capacity_units NUMERIC(18,3),
    ADD COLUMN IF NOT EXISTS transit_hours NUMERIC(10,2),
    ADD COLUMN IF NOT EXISTS sla_score_input NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS carrier_kpi_score_input NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS reliability_score_input NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS active_revision_number INTEGER NOT NULL DEFAULT 1;

CREATE TABLE rfx.rfx_response_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_response_id UUID NOT NULL REFERENCES rfx.rfx_responses(id) ON DELETE CASCADE,
    revision_number INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    price_amount NUMERIC(18,2),
    currency_code CHAR(3) DEFAULT 'RUB',
    capacity_units NUMERIC(18,3),
    transit_hours NUMERIC(10,2),
    sla_score_input NUMERIC(5,2),
    carrier_kpi_score_input NUMERIC(5,2),
    reliability_score_input NUMERIC(5,2),
    comment TEXT,
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    idempotency_key VARCHAR(200),
    CONSTRAINT uq_rfx_response_revision UNIQUE (rfx_response_id, revision_number),
    CONSTRAINT uq_rfx_response_revision_idempotency UNIQUE (tenant_id, idempotency_key)
);

CREATE UNIQUE INDEX uq_rfx_response_active_revision ON rfx.rfx_response_revisions(rfx_response_id) WHERE is_active = true;

-- Extend bids with evaluation inputs and idempotency
ALTER TABLE rfx.bids
    ADD COLUMN IF NOT EXISTS capacity_units NUMERIC(18,3),
    ADD COLUMN IF NOT EXISTS transit_hours NUMERIC(10,2),
    ADD COLUMN IF NOT EXISTS sla_score_input NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS carrier_kpi_score_input NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS reliability_score_input NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(200),
    ADD COLUMN IF NOT EXISTS active_revision_number INTEGER NOT NULL DEFAULT 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_bid_idempotency ON rfx.bids(tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Optional link freight request to enterprise RFx event
ALTER TABLE rfx.freight_requests
    ADD COLUMN IF NOT EXISTS rfx_event_id UUID REFERENCES rfx.rfx_events(id);

-- Extend rfx_events with evaluation lifecycle statuses (DB already supports them)
-- Add tender evaluation config reference
ALTER TABLE rfx.rfx_events
    ADD COLUMN IF NOT EXISTS scoring_template_version_id UUID REFERENCES rfx.scoring_template_versions(id),
    ADD COLUMN IF NOT EXISTS bidding_closed_at TIMESTAMPTZ;

INSERT INTO core.permissions (code, resource, action, description)
VALUES
    ('rfx.evaluate', 'rfx', 'evaluate', 'Run tender evaluation, scoring and allocation'),
    ('rfx.approve_award', 'rfx', 'approve_award', 'Approve tender award proposals')
ON CONFLICT (code) DO NOTHING;
