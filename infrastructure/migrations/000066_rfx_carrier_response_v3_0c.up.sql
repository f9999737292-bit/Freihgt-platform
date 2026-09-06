-- RFx v3.0C — carrier questionnaire answer persistence and response metadata.

ALTER TABLE rfx.rfx_responses
    ADD COLUMN IF NOT EXISTS rfx_version_id UUID REFERENCES rfx.rfx_versions(id),
    ADD COLUMN IF NOT EXISTS save_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_saved_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_saved_by UUID,
    ADD COLUMN IF NOT EXISTS completion_percent NUMERIC(5,2) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_rfx_responses_version_id
    ON rfx.rfx_responses(tenant_id, rfx_version_id)
    WHERE rfx_version_id IS NOT NULL;

CREATE TABLE rfx.rfx_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rfx_response_id UUID NOT NULL REFERENCES rfx.rfx_responses(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES rfx.rfx_questions(id),
    answer_value_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    answer_source VARCHAR(64) NOT NULL DEFAULT 'CARRIER_DECLARED',
    validation_version INT NOT NULL DEFAULT 1,
    rule_version INT,
    updated_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_rfx_answers_response_question UNIQUE (rfx_response_id, question_id)
);

CREATE INDEX idx_rfx_answers_tenant_response
    ON rfx.rfx_answers(tenant_id, rfx_response_id);

CREATE INDEX idx_rfx_answers_question
    ON rfx.rfx_answers(question_id);
