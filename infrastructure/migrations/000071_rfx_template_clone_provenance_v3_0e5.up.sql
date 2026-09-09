-- RFx v3.0E5 — template clone provenance on rfx_events.

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_template_versions_tenant_version_id
    ON rfx.rfx_template_versions (tenant_id, id);

ALTER TABLE rfx.rfx_events
    ADD COLUMN IF NOT EXISTS source_template_version_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_rfx_events_source_template_version_id
    ON rfx.rfx_events (source_template_version_id)
    WHERE source_template_version_id IS NOT NULL;

ALTER TABLE rfx.rfx_events
    ADD CONSTRAINT fk_rfx_events_source_template_version_composite
    FOREIGN KEY (tenant_id, source_template_version_id)
    REFERENCES rfx.rfx_template_versions (tenant_id, id);

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

DROP TRIGGER IF EXISTS trg_rfx_events_provenance_immutable ON rfx.rfx_events;

CREATE TRIGGER trg_rfx_events_provenance_immutable
    BEFORE UPDATE ON rfx.rfx_events
    FOR EACH ROW
    EXECUTE FUNCTION rfx.prevent_rfx_event_provenance_mutation();
