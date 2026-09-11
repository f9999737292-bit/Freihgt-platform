SET search_path = public;

DROP TRIGGER IF EXISTS trg_rfx_events_creation_channel_on_insert ON rfx.rfx_events;

DROP FUNCTION IF EXISTS rfx.set_rfx_event_creation_channel_on_insert();

CREATE OR REPLACE FUNCTION rfx.prevent_rfx_event_provenance_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE'
        AND NEW.source_template_version_id IS DISTINCT FROM OLD.source_template_version_id THEN
        RAISE EXCEPTION 'source_template_version_id is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP INDEX IF EXISTS rfx.idx_rfx_events_creation_channel;

ALTER TABLE rfx.rfx_events
    DROP CONSTRAINT IF EXISTS chk_rfx_event_creation_channel_template_consistency;

ALTER TABLE rfx.rfx_events
    DROP CONSTRAINT IF EXISTS chk_rfx_event_creation_channel;

ALTER TABLE rfx.rfx_events
    DROP COLUMN IF EXISTS creation_channel;

DROP TRIGGER IF EXISTS trg_rfx_external_object_links_updated_at ON rfx.rfx_external_object_links;

ALTER TABLE rfx.rfx_external_object_links
    DROP CONSTRAINT IF EXISTS fk_rfx_external_object_link_event_composite;

DROP INDEX IF EXISTS rfx.idx_rfx_external_object_links_event;
DROP INDEX IF EXISTS rfx.uq_rfx_external_object_identity;

DROP TABLE IF EXISTS rfx.rfx_external_object_links;

DROP TRIGGER IF EXISTS trg_rfx_import_analyses_payload_immutable ON rfx.rfx_import_analyses;

DROP FUNCTION IF EXISTS rfx.prevent_rfx_import_analysis_payload_mutation();

DROP INDEX IF EXISTS rfx.idx_rfx_import_analyses_target;
DROP INDEX IF EXISTS rfx.idx_rfx_import_analyses_expires_at;
DROP INDEX IF EXISTS rfx.idx_rfx_import_analyses_tenant_company;
DROP INDEX IF EXISTS rfx.idx_rfx_import_analyses_tenant_actor;

DROP TABLE IF EXISTS rfx.rfx_import_analyses;
