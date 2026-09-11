-- RFx v3.0E7 Phase 2 — Excel/ERP exchange persistence (import preview analyses + external links + creation channel).

CREATE TABLE IF NOT EXISTS rfx.rfx_import_analyses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    actor_company_id UUID NOT NULL,
    workbook_type VARCHAR(64) NOT NULL,
    schema_version VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id UUID NULL,
    target_version INTEGER NULL,
    canonical_payload_json JSONB NOT NULL,
    canonical_hash CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'PREVIEWED',
    validation_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NULL,
    result_reference_type VARCHAR(32) NULL,
    result_reference_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_import_analysis_status CHECK (
        status IN ('PREVIEWED', 'CONSUMED', 'EXPIRED')
    ),
    CONSTRAINT chk_rfx_import_analysis_workbook_type CHECK (
        workbook_type IN ('BUYER_TENDER', 'CARRIER_OFFER')
    ),
    CONSTRAINT chk_rfx_import_analysis_schema_version CHECK (
        schema_version IN ('BINTRANS_RFX_BUYER_XLSX_V1', 'BINTRANS_RFX_CARRIER_XLSX_V1')
    ),
    CONSTRAINT chk_rfx_import_analysis_target_type CHECK (
        target_type IN ('NEW_EVENT', 'DRAFT_EVENT', 'CARRIER_RESPONSE')
    ),
    CONSTRAINT chk_rfx_import_analysis_target_version CHECK (
        target_version IS NULL OR target_version > 0
    ),
    CONSTRAINT chk_rfx_import_analysis_consumed CHECK (
        (
            status = 'CONSUMED'
            AND consumed_at IS NOT NULL
            AND result_reference_type IS NOT NULL
            AND result_reference_id IS NOT NULL
        )
        OR (
            status <> 'CONSUMED'
            AND consumed_at IS NULL
            AND result_reference_type IS NULL
            AND result_reference_id IS NULL
        )
    )
);

CREATE INDEX IF NOT EXISTS idx_rfx_import_analyses_tenant_actor
    ON rfx.rfx_import_analyses (tenant_id, actor_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_rfx_import_analyses_tenant_company
    ON rfx.rfx_import_analyses (tenant_id, actor_company_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_rfx_import_analyses_expires_at
    ON rfx.rfx_import_analyses (expires_at)
    WHERE status = 'PREVIEWED';

CREATE INDEX IF NOT EXISTS idx_rfx_import_analyses_target
    ON rfx.rfx_import_analyses (tenant_id, target_type, target_id)
    WHERE target_id IS NOT NULL;

CREATE OR REPLACE FUNCTION rfx.prevent_rfx_import_analysis_payload_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NEW.canonical_payload_json IS DISTINCT FROM OLD.canonical_payload_json
            OR NEW.canonical_hash IS DISTINCT FROM OLD.canonical_hash
            OR NEW.workbook_type IS DISTINCT FROM OLD.workbook_type
            OR NEW.schema_version IS DISTINCT FROM OLD.schema_version
            OR NEW.target_type IS DISTINCT FROM OLD.target_type
            OR NEW.target_id IS DISTINCT FROM OLD.target_id
            OR NEW.target_version IS DISTINCT FROM OLD.target_version
            OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
            OR NEW.actor_id IS DISTINCT FROM OLD.actor_id
            OR NEW.actor_company_id IS DISTINCT FROM OLD.actor_company_id THEN
            RAISE EXCEPTION 'import analysis preview payload is immutable';
        END IF;
        IF OLD.status = 'CONSUMED' AND NEW.status <> 'CONSUMED' THEN
            RAISE EXCEPTION 'import analysis is single-use';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_rfx_import_analyses_payload_immutable ON rfx.rfx_import_analyses;

CREATE TRIGGER trg_rfx_import_analyses_payload_immutable
    BEFORE UPDATE ON rfx.rfx_import_analyses
    FOR EACH ROW
    EXECUTE FUNCTION rfx.prevent_rfx_import_analysis_payload_mutation();

CREATE TABLE IF NOT EXISTS rfx.rfx_external_object_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    integration_principal_id UUID NOT NULL,
    external_system VARCHAR(64) NOT NULL,
    external_object_type VARCHAR(64) NOT NULL,
    external_object_id VARCHAR(256) NOT NULL,
    external_version VARCHAR(64) NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    rfx_event_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_external_object_system_nonempty CHECK (length(trim(external_system)) > 0),
    CONSTRAINT chk_rfx_external_object_type_nonempty CHECK (length(trim(external_object_type)) > 0),
    CONSTRAINT chk_rfx_external_object_id_nonempty CHECK (length(trim(external_object_id)) > 0),
    CONSTRAINT chk_rfx_external_object_version_nonempty CHECK (length(trim(external_version)) > 0),
    CONSTRAINT chk_rfx_external_object_payload_hash CHECK (length(trim(payload_hash)) = 64)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_external_object_identity
    ON rfx.rfx_external_object_links (
        tenant_id,
        integration_principal_id,
        external_system,
        external_object_type,
        external_object_id,
        external_version
    );

CREATE INDEX IF NOT EXISTS idx_rfx_external_object_links_event
    ON rfx.rfx_external_object_links (tenant_id, rfx_event_id);

ALTER TABLE rfx.rfx_external_object_links
    ADD CONSTRAINT fk_rfx_external_object_link_event_composite
    FOREIGN KEY (tenant_id, rfx_event_id)
    REFERENCES rfx.rfx_events (tenant_id, id);

DROP TRIGGER IF EXISTS trg_rfx_external_object_links_updated_at ON rfx.rfx_external_object_links;

CREATE TRIGGER trg_rfx_external_object_links_updated_at
    BEFORE UPDATE ON rfx.rfx_external_object_links
    FOR EACH ROW
    EXECUTE FUNCTION core.set_updated_at();

ALTER TABLE rfx.rfx_events
    ADD COLUMN IF NOT EXISTS creation_channel VARCHAR(16) NOT NULL DEFAULT 'MANUAL';

UPDATE rfx.rfx_events
SET creation_channel = 'TEMPLATE'
WHERE source_template_version_id IS NOT NULL;

ALTER TABLE rfx.rfx_events
    ADD CONSTRAINT chk_rfx_event_creation_channel CHECK (
        creation_channel IN ('MANUAL', 'TEMPLATE', 'EXCEL', 'ERP')
    );

ALTER TABLE rfx.rfx_events
    ADD CONSTRAINT chk_rfx_event_creation_channel_template_consistency CHECK (
        source_template_version_id IS NULL OR creation_channel = 'TEMPLATE'
    );

CREATE INDEX IF NOT EXISTS idx_rfx_events_creation_channel
    ON rfx.rfx_events (tenant_id, creation_channel);

CREATE OR REPLACE FUNCTION rfx.set_rfx_event_creation_channel_on_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.source_template_version_id IS NOT NULL THEN
        NEW.creation_channel := 'TEMPLATE';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_rfx_events_creation_channel_on_insert ON rfx.rfx_events;

CREATE TRIGGER trg_rfx_events_creation_channel_on_insert
    BEFORE INSERT ON rfx.rfx_events
    FOR EACH ROW
    EXECUTE FUNCTION rfx.set_rfx_event_creation_channel_on_insert();

CREATE OR REPLACE FUNCTION rfx.prevent_rfx_event_provenance_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NEW.source_template_version_id IS DISTINCT FROM OLD.source_template_version_id THEN
            RAISE EXCEPTION 'source_template_version_id is immutable';
        END IF;
        IF NEW.creation_channel IS DISTINCT FROM OLD.creation_channel THEN
            RAISE EXCEPTION 'creation_channel is immutable';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DO $$
DECLARE
    ambiguous_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO ambiguous_count
    FROM rfx.rfx_events
    WHERE source_template_version_id IS NOT NULL
      AND creation_channel <> 'TEMPLATE';

    IF ambiguous_count > 0 THEN
        RAISE EXCEPTION 'ambiguous creation_channel backfill: % rows', ambiguous_count;
    END IF;
END;
$$;
