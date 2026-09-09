DROP TRIGGER IF EXISTS trg_rfx_events_provenance_immutable ON rfx.rfx_events;

DROP FUNCTION IF EXISTS rfx.prevent_rfx_event_provenance_mutation();

ALTER TABLE rfx.rfx_events
    DROP CONSTRAINT IF EXISTS fk_rfx_events_source_template_version_composite;

DROP INDEX IF EXISTS rfx.idx_rfx_events_source_template_version_id;

ALTER TABLE rfx.rfx_events
    DROP COLUMN IF EXISTS source_template_version_id;

DROP INDEX IF EXISTS rfx.uq_rfx_template_versions_tenant_version_id;
