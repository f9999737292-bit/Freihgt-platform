SET search_path = public;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM rfx.rfx_idempotency_records
        WHERE integration_principal_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'down migration blocked: ERP idempotency rows exist';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM rfx.rfx_import_analyses
        WHERE integration_principal_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'down migration blocked: ERP-owned import analyses exist';
    END IF;

    IF EXISTS (
        SELECT 1 FROM rfx.rfx_reference_mapping_sets
    ) THEN
        RAISE EXCEPTION 'down migration blocked: reference mapping sets exist';
    END IF;

    IF EXISTS (
        SELECT 1 FROM rfx.rfx_external_object_link_revisions
    ) THEN
        RAISE EXCEPTION 'down migration blocked: external object link revisions exist';
    END IF;
END;
$$;

DROP INDEX IF EXISTS rfx.idx_rfx_reference_mapping_entries_tenant_set;
DROP INDEX IF EXISTS rfx.uq_rfx_reference_mapping_entries_set_external_code;
DROP TABLE IF EXISTS rfx.rfx_reference_mapping_entries;

DROP TRIGGER IF EXISTS trg_rfx_reference_mapping_sets_updated_at ON rfx.rfx_reference_mapping_sets;
DROP INDEX IF EXISTS rfx.idx_rfx_reference_mapping_sets_lookup;
DROP INDEX IF EXISTS rfx.uq_rfx_reference_mapping_sets_platform_type_version;
DROP INDEX IF EXISTS rfx.uq_rfx_reference_mapping_sets_tenant_type_version;
DROP TABLE IF EXISTS rfx.rfx_reference_mapping_sets;

DROP INDEX IF EXISTS rfx.idx_rfx_external_object_link_revisions_tenant_link;
DROP INDEX IF EXISTS rfx.uq_rfx_external_object_link_revision;
DROP TABLE IF EXISTS rfx.rfx_external_object_link_revisions;

ALTER TABLE rfx.rfx_external_object_links
    DROP CONSTRAINT IF EXISTS fk_rfx_external_object_link_integration_principal;

DROP INDEX IF EXISTS rfx.uq_rfx_external_object_stable_identity;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_external_object_identity
    ON rfx.rfx_external_object_links (
        tenant_id,
        integration_principal_id,
        external_system,
        external_object_type,
        external_object_id,
        external_version
    );

ALTER TABLE rfx.rfx_external_object_links
    DROP COLUMN IF EXISTS external_revision;

ALTER TABLE rfx.rfx_idempotency_records
    DROP CONSTRAINT IF EXISTS fk_rfx_idempotency_integration_principal;

DROP INDEX IF EXISTS rfx.idx_rfx_idempotency_erp_principal;
DROP INDEX IF EXISTS rfx.uq_rfx_idempotency_erp_scope_key;
DROP INDEX IF EXISTS rfx.uq_rfx_idempotency_human_scope_key;

ALTER TABLE rfx.rfx_idempotency_records
    DROP CONSTRAINT IF EXISTS chk_rfx_idempotency_owner_xor;

ALTER TABLE rfx.rfx_idempotency_records
    DROP COLUMN IF EXISTS integration_principal_id;

ALTER TABLE rfx.rfx_idempotency_records
    ALTER COLUMN actor_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_idempotency_scope_key
    ON rfx.rfx_idempotency_records (
        tenant_id, actor_id, operation, aggregate_scope, idempotency_key
    );

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

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS fk_rfx_import_analysis_integration_principal;

DROP INDEX IF EXISTS rfx.idx_rfx_import_analyses_tenant_principal;

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS chk_rfx_import_analysis_owner_xor;

ALTER TABLE rfx.rfx_import_analyses
    DROP COLUMN IF EXISTS integration_principal_id;

ALTER TABLE rfx.rfx_import_analyses
    ALTER COLUMN actor_id SET NOT NULL;

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS chk_rfx_import_analysis_workbook_type;

ALTER TABLE rfx.rfx_import_analyses
    ADD CONSTRAINT chk_rfx_import_analysis_workbook_type CHECK (
        workbook_type IN ('BUYER_TENDER', 'CARRIER_OFFER')
    );

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS chk_rfx_import_analysis_schema_version;

ALTER TABLE rfx.rfx_import_analyses
    ADD CONSTRAINT chk_rfx_import_analysis_schema_version CHECK (
        schema_version IN ('BINTRANS_RFX_BUYER_XLSX_V1', 'BINTRANS_RFX_CARRIER_XLSX_V1')
    );

DROP TRIGGER IF EXISTS trg_rfx_integration_scopes_updated_at ON rfx.rfx_integration_scopes;
DROP INDEX IF EXISTS rfx.idx_rfx_integration_scopes_tenant_principal;
DROP INDEX IF EXISTS rfx.uq_rfx_integration_scopes_principal_scope;
DROP TABLE IF EXISTS rfx.rfx_integration_scopes;

DROP TRIGGER IF EXISTS trg_rfx_integration_credentials_updated_at ON rfx.rfx_integration_credentials;
DROP INDEX IF EXISTS rfx.idx_rfx_integration_credentials_principal;
DROP INDEX IF EXISTS rfx.uq_rfx_integration_credentials_lookup_fingerprint;
DROP TABLE IF EXISTS rfx.rfx_integration_credentials;

DROP TRIGGER IF EXISTS trg_rfx_integration_principals_updated_at ON rfx.rfx_integration_principals;
DROP INDEX IF EXISTS rfx.idx_rfx_integration_principals_tenant_status;
DROP INDEX IF EXISTS rfx.idx_rfx_integration_principals_tenant_company;
DROP INDEX IF EXISTS rfx.uq_rfx_integration_principals_tenant_client_id;
DROP TABLE IF EXISTS rfx.rfx_integration_principals;
