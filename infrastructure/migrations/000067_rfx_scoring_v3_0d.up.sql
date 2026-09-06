-- RFx v3.0D — automatic scoring, knockout, and qualification results.

CREATE TABLE rfx.rfx_score_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_version_id UUID NOT NULL REFERENCES rfx.rfx_versions(id) ON DELETE CASCADE,
    model_version INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'DRAFT',
    model_type VARCHAR(32) NOT NULL DEFAULT 'AUTOMATIC',
    definition_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    CONSTRAINT uq_rfx_score_models_version UNIQUE (rfx_version_id, model_version),
    CONSTRAINT chk_rfx_score_models_status CHECK (status IN ('DRAFT', 'PUBLISHED')),
    CONSTRAINT chk_rfx_score_models_type CHECK (model_type IN ('AUTOMATIC', 'MANUAL', 'HYBRID', 'SYSTEM_DERIVED'))
);

CREATE UNIQUE INDEX uq_rfx_score_models_published_version
    ON rfx.rfx_score_models (rfx_version_id)
    WHERE status = 'PUBLISHED';

CREATE INDEX idx_rfx_score_models_tenant_version
    ON rfx.rfx_score_models (tenant_id, rfx_version_id);

CREATE TABLE rfx.rfx_score_criteria (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    score_model_id UUID NOT NULL REFERENCES rfx.rfx_score_models(id) ON DELETE CASCADE,
    criterion_code VARCHAR(128) NOT NULL,
    name VARCHAR(512) NOT NULL,
    weight NUMERIC(8, 4) NOT NULL,
    normalization_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rfx_score_criteria_code UNIQUE (score_model_id, criterion_code),
    CONSTRAINT chk_rfx_score_criteria_weight CHECK (weight >= 0)
);

CREATE INDEX idx_rfx_score_criteria_model
    ON rfx.rfx_score_criteria (tenant_id, score_model_id);

CREATE TABLE rfx.rfx_score_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    score_model_id UUID NOT NULL REFERENCES rfx.rfx_score_models(id) ON DELETE CASCADE,
    criterion_id UUID NOT NULL REFERENCES rfx.rfx_score_criteria(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES rfx.rfx_questions(id),
    binding_type VARCHAR(64) NOT NULL DEFAULT 'QUESTION',
    scoring_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    knockout_rule_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rfx_score_bindings_criterion_question UNIQUE (score_model_id, criterion_id, question_id)
);

CREATE INDEX idx_rfx_score_bindings_model
    ON rfx.rfx_score_bindings (tenant_id, score_model_id);

CREATE INDEX idx_rfx_score_bindings_question
    ON rfx.rfx_score_bindings (question_id);

CREATE TABLE rfx.rfx_answer_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_response_id UUID NOT NULL REFERENCES rfx.rfx_responses(id) ON DELETE CASCADE,
    answer_id UUID NOT NULL REFERENCES rfx.rfx_answers(id) ON DELETE CASCADE,
    criterion_id UUID NOT NULL REFERENCES rfx.rfx_score_criteria(id),
    score_model_id UUID NOT NULL REFERENCES rfx.rfx_score_models(id),
    score_model_version INT NOT NULL,
    raw_score NUMERIC(12, 4) NOT NULL,
    normalized_score NUMERIC(12, 4) NOT NULL,
    weighted_contribution NUMERIC(12, 4) NOT NULL,
    explanation_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rfx_answer_scores_identity UNIQUE (rfx_response_id, answer_id, criterion_id, score_model_version)
);

CREATE INDEX idx_rfx_answer_scores_response
    ON rfx.rfx_answer_scores (tenant_id, rfx_response_id, score_model_version);

CREATE TABLE rfx.rfx_qualification_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_response_id UUID NOT NULL REFERENCES rfx.rfx_responses(id) ON DELETE CASCADE,
    score_model_id UUID NOT NULL REFERENCES rfx.rfx_score_models(id),
    score_model_version INT NOT NULL,
    status VARCHAR(32) NOT NULL,
    calculation_status VARCHAR(32) NOT NULL DEFAULT 'CALCULATED',
    total_score NUMERIC(12, 4),
    knockout_triggered BOOLEAN NOT NULL DEFAULT FALSE,
    knockout_reason_json JSONB,
    calculated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_rfx_qualification_results_identity UNIQUE (rfx_response_id, score_model_version),
    CONSTRAINT chk_rfx_qualification_status CHECK (status IN ('QUALIFIED', 'CONDITIONALLY_QUALIFIED', 'REJECTED', 'PENDING_REVIEW')),
    CONSTRAINT chk_rfx_qualification_calculation CHECK (calculation_status IN ('PENDING', 'CALCULATED', 'FAILED'))
);

CREATE INDEX idx_rfx_qualification_results_response
    ON rfx.rfx_qualification_results (tenant_id, rfx_response_id);
