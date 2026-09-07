DROP INDEX IF EXISTS idx_rfx_idempotency_expires_at;
DROP INDEX IF EXISTS uq_rfx_idempotency_scope_key;
DROP TABLE IF EXISTS rfx.rfx_idempotency_records;

ALTER TABLE rfx.rfx_versions
    DROP CONSTRAINT IF EXISTS fk_rfx_versions_superseded_by_composite;

ALTER TABLE rfx.rfx_events
    DROP CONSTRAINT IF EXISTS fk_rfx_events_published_version_composite;

DROP INDEX IF EXISTS idx_rfx_events_published_version_id;
ALTER TABLE rfx.rfx_events
    DROP COLUMN IF EXISTS published_version_id;

ALTER TABLE rfx.rfx_versions
    DROP CONSTRAINT IF EXISTS chk_rfx_versions_no_self_supersede;

DROP INDEX IF EXISTS uq_rfx_versions_tenant_event_id;

DROP INDEX IF EXISTS uq_rfx_versions_one_published_per_event;
DROP INDEX IF EXISTS uq_rfx_versions_one_draft_per_event;

ALTER TABLE rfx.rfx_versions
    DROP COLUMN IF EXISTS rescoring_required,
    DROP COLUMN IF EXISTS superseded_by_version_id,
    DROP COLUMN IF EXISTS superseded_at,
    DROP COLUMN IF EXISTS change_summary;
