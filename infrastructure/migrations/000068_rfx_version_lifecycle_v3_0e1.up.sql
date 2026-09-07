-- RFx v3.0E1 - event questionnaire version lifecycle foundation.

ALTER TABLE rfx.rfx_versions
    ADD COLUMN IF NOT EXISTS change_summary TEXT,
    ADD COLUMN IF NOT EXISTS superseded_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS superseded_by_version_id UUID REFERENCES rfx.rfx_versions(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS rescoring_required BOOLEAN NOT NULL DEFAULT FALSE;

DROP INDEX IF EXISTS uq_rfx_versions_one_draft_per_event;
CREATE UNIQUE INDEX uq_rfx_versions_one_draft_per_event
    ON rfx.rfx_versions (rfx_event_id)
    WHERE status = 'DRAFT' AND deleted_at IS NULL;

DROP INDEX IF EXISTS uq_rfx_versions_one_published_per_event;
CREATE UNIQUE INDEX uq_rfx_versions_one_published_per_event
    ON rfx.rfx_versions (rfx_event_id)
    WHERE status = 'PUBLISHED' AND deleted_at IS NULL;

ALTER TABLE rfx.rfx_events
    ADD COLUMN IF NOT EXISTS published_version_id UUID REFERENCES rfx.rfx_versions(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM rfx.rfx_versions
        WHERE status = 'PUBLISHED' AND deleted_at IS NULL
        GROUP BY rfx_event_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'migration 000068 failed: multiple PUBLISHED questionnaire versions exist for at least one RFx event';
    END IF;
END $$;

UPDATE rfx.rfx_events e
SET published_version_id = v.id
FROM rfx.rfx_versions v
WHERE v.rfx_event_id = e.id
  AND v.tenant_id = e.tenant_id
  AND v.status = 'PUBLISHED'
  AND v.deleted_at IS NULL
  AND e.published_version_id IS DISTINCT FROM v.id;

CREATE INDEX IF NOT EXISTS idx_rfx_events_published_version_id
    ON rfx.rfx_events(published_version_id);

CREATE TABLE IF NOT EXISTS rfx.rfx_idempotency_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    operation VARCHAR(64) NOT NULL,
    aggregate_scope UUID NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    request_body_hash VARCHAR(64) NOT NULL,
    response_status INT NOT NULL,
    response_body_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '24 hours')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_idempotency_scope_key
    ON rfx.rfx_idempotency_records (tenant_id, actor_id, operation, aggregate_scope, idempotency_key);

CREATE INDEX IF NOT EXISTS idx_rfx_idempotency_expires_at
    ON rfx.rfx_idempotency_records (expires_at);
