-- RFx v3.0E3 - change impact analysis persistence for publish confirmation.

CREATE TABLE IF NOT EXISTS rfx.rfx_change_impact_analyses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    event_id UUID NOT NULL,
    source_version_id UUID NULL,
    candidate_version_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    canonical_diff_hash VARCHAR(64) NOT NULL,
    impact_classes JSONB NOT NULL,
    affected_draft_response_count INT NOT NULL DEFAULT 0,
    affected_submitted_response_count INT NOT NULL DEFAULT 0,
    scoring_affecting BOOLEAN NOT NULL DEFAULT FALSE,
    knockout_affecting BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_rfx_change_impact_draft_count_nonneg CHECK (affected_draft_response_count >= 0),
    CONSTRAINT chk_rfx_change_impact_submitted_count_nonneg CHECK (affected_submitted_response_count >= 0),
    CONSTRAINT chk_rfx_change_impact_classes_array CHECK (jsonb_typeof(impact_classes) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_rfx_change_impact_event_candidate_created
    ON rfx.rfx_change_impact_analyses (tenant_id, event_id, candidate_version_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_rfx_change_impact_expires_unconsumed
    ON rfx.rfx_change_impact_analyses (expires_at)
    WHERE consumed_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_events_tenant_id
    ON rfx.rfx_events (tenant_id, id);

ALTER TABLE rfx.rfx_change_impact_analyses
    ADD CONSTRAINT fk_rfx_change_impact_event_composite
    FOREIGN KEY (tenant_id, event_id)
    REFERENCES rfx.rfx_events (tenant_id, id);

ALTER TABLE rfx.rfx_change_impact_analyses
    ADD CONSTRAINT fk_rfx_change_impact_candidate_composite
    FOREIGN KEY (tenant_id, event_id, candidate_version_id)
    REFERENCES rfx.rfx_versions (tenant_id, rfx_event_id, id);

ALTER TABLE rfx.rfx_change_impact_analyses
    ADD CONSTRAINT fk_rfx_change_impact_source_composite
    FOREIGN KEY (tenant_id, event_id, source_version_id)
    REFERENCES rfx.rfx_versions (tenant_id, rfx_event_id, id);
