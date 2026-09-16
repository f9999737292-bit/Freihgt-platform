-- RFx v3.0E7 Phase 2 — ERP integration principals, credentials, scopes, reference mapping,
-- analysis/idempotency owner XOR, stable external identity (ADR-015).

CREATE TABLE IF NOT EXISTS rfx.rfx_integration_principals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants (id),
    company_id UUID NOT NULL,
    client_id VARCHAR(128) NOT NULL,
    credential_type VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    allowed_cidrs JSONB NULL,
    revoked_at TIMESTAMPTZ NULL,
    grace_ends_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_integration_principal_credential_type CHECK (
        credential_type IN ('OAUTH', 'API_KEY')
    ),
    CONSTRAINT chk_rfx_integration_principal_status CHECK (
        status IN ('ACTIVE', 'REVOKED', 'SUSPENDED')
    ),
    CONSTRAINT chk_rfx_integration_principal_client_id_nonempty CHECK (
        length(trim(client_id)) > 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_integration_principals_tenant_client_id
    ON rfx.rfx_integration_principals (tenant_id, lower(trim(client_id)));

CREATE INDEX IF NOT EXISTS idx_rfx_integration_principals_tenant_company
    ON rfx.rfx_integration_principals (tenant_id, company_id);

CREATE INDEX IF NOT EXISTS idx_rfx_integration_principals_tenant_status
    ON rfx.rfx_integration_principals (tenant_id, status);

DROP TRIGGER IF EXISTS trg_rfx_integration_principals_updated_at ON rfx.rfx_integration_principals;

CREATE TRIGGER trg_rfx_integration_principals_updated_at
    BEFORE UPDATE ON rfx.rfx_integration_principals
    FOR EACH ROW
    EXECUTE FUNCTION core.set_updated_at();

CREATE TABLE IF NOT EXISTS rfx.rfx_integration_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants (id),
    integration_principal_id UUID NOT NULL REFERENCES rfx.rfx_integration_principals (id),
    credential_type VARCHAR(16) NOT NULL,
    secret_hash VARCHAR(255) NOT NULL,
    hash_algorithm VARCHAR(32) NOT NULL,
    hash_version INTEGER NOT NULL DEFAULT 1,
    lookup_fingerprint VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    rotation_predecessor_id UUID NULL REFERENCES rfx.rfx_integration_credentials (id),
    rotation_successor_id UUID NULL REFERENCES rfx.rfx_integration_credentials (id),
    display_once_acknowledged_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_integration_credential_type CHECK (
        credential_type IN ('OAUTH', 'API_KEY')
    ),
    CONSTRAINT chk_rfx_integration_credential_secret_hash_nonempty CHECK (
        length(trim(secret_hash)) > 0
    ),
    CONSTRAINT chk_rfx_integration_credential_hash_algorithm_nonempty CHECK (
        length(trim(hash_algorithm)) > 0
    ),
    CONSTRAINT chk_rfx_integration_credential_lookup_fingerprint_nonempty CHECK (
        length(trim(lookup_fingerprint)) > 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_integration_credentials_lookup_fingerprint
    ON rfx.rfx_integration_credentials (tenant_id, lookup_fingerprint);

CREATE INDEX IF NOT EXISTS idx_rfx_integration_credentials_principal
    ON rfx.rfx_integration_credentials (integration_principal_id, issued_at DESC);

DROP TRIGGER IF EXISTS trg_rfx_integration_credentials_updated_at ON rfx.rfx_integration_credentials;

CREATE TRIGGER trg_rfx_integration_credentials_updated_at
    BEFORE UPDATE ON rfx.rfx_integration_credentials
    FOR EACH ROW
    EXECUTE FUNCTION core.set_updated_at();

CREATE TABLE IF NOT EXISTS rfx.rfx_integration_scopes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants (id),
    integration_principal_id UUID NOT NULL REFERENCES rfx.rfx_integration_principals (id),
    scope VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_integration_scope_value CHECK (
        scope IN (
            'rfx:draft:create',
            'rfx:draft:read',
            'rfx:draft:preview',
            'rfx:draft:commit',
            'rfx:status:read'
        )
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_integration_scopes_principal_scope
    ON rfx.rfx_integration_scopes (integration_principal_id, scope);

CREATE INDEX IF NOT EXISTS idx_rfx_integration_scopes_tenant_principal
    ON rfx.rfx_integration_scopes (tenant_id, integration_principal_id);

ALTER TABLE rfx.rfx_import_analyses
    ADD COLUMN IF NOT EXISTS integration_principal_id UUID NULL;

ALTER TABLE rfx.rfx_import_analyses
    ALTER COLUMN actor_id DROP NOT NULL;

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS chk_rfx_import_analysis_workbook_type;

ALTER TABLE rfx.rfx_import_analyses
    ADD CONSTRAINT chk_rfx_import_analysis_workbook_type CHECK (
        workbook_type IN ('BUYER_TENDER', 'CARRIER_OFFER', 'ERP_BUYER_JSON')
    );

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS chk_rfx_import_analysis_schema_version;

ALTER TABLE rfx.rfx_import_analyses
    ADD CONSTRAINT chk_rfx_import_analysis_schema_version CHECK (
        schema_version IN (
            'BINTRANS_RFX_BUYER_XLSX_V1',
            'BINTRANS_RFX_CARRIER_XLSX_V1',
            'BINTRANS_RFX_ERP_JSON_V1'
        )
    );

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS chk_rfx_import_analysis_owner_xor;

ALTER TABLE rfx.rfx_import_analyses
    ADD CONSTRAINT chk_rfx_import_analysis_owner_xor CHECK (
        (
            actor_id IS NOT NULL
            AND integration_principal_id IS NULL
        )
        OR (
            actor_id IS NULL
            AND integration_principal_id IS NOT NULL
        )
    );

ALTER TABLE rfx.rfx_import_analyses
    DROP CONSTRAINT IF EXISTS fk_rfx_import_analysis_integration_principal;

ALTER TABLE rfx.rfx_import_analyses
    ADD CONSTRAINT fk_rfx_import_analysis_integration_principal
    FOREIGN KEY (integration_principal_id)
    REFERENCES rfx.rfx_integration_principals (id);

CREATE INDEX IF NOT EXISTS idx_rfx_import_analyses_tenant_principal
    ON rfx.rfx_import_analyses (tenant_id, integration_principal_id, created_at DESC)
    WHERE integration_principal_id IS NOT NULL;

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
            OR NEW.actor_company_id IS DISTINCT FROM OLD.actor_company_id
            OR NEW.integration_principal_id IS DISTINCT FROM OLD.integration_principal_id THEN
            RAISE EXCEPTION 'import analysis preview payload is immutable';
        END IF;
        IF OLD.status = 'CONSUMED' AND NEW.status <> 'CONSUMED' THEN
            RAISE EXCEPTION 'import analysis is single-use';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

ALTER TABLE rfx.rfx_idempotency_records
    ADD COLUMN IF NOT EXISTS integration_principal_id UUID NULL;

ALTER TABLE rfx.rfx_idempotency_records
    ALTER COLUMN actor_id DROP NOT NULL;

ALTER TABLE rfx.rfx_idempotency_records
    DROP CONSTRAINT IF EXISTS chk_rfx_idempotency_owner_xor;

ALTER TABLE rfx.rfx_idempotency_records
    ADD CONSTRAINT chk_rfx_idempotency_owner_xor CHECK (
        (
            actor_id IS NOT NULL
            AND integration_principal_id IS NULL
        )
        OR (
            actor_id IS NULL
            AND integration_principal_id IS NOT NULL
        )
    );

DROP INDEX IF EXISTS rfx.uq_rfx_idempotency_scope_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_idempotency_human_scope_key
    ON rfx.rfx_idempotency_records (
        tenant_id, actor_id, operation, aggregate_scope, idempotency_key
    )
    WHERE actor_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_idempotency_erp_scope_key
    ON rfx.rfx_idempotency_records (
        tenant_id, integration_principal_id, operation, aggregate_scope, idempotency_key
    )
    WHERE integration_principal_id IS NOT NULL;

ALTER TABLE rfx.rfx_idempotency_records
    DROP CONSTRAINT IF EXISTS fk_rfx_idempotency_integration_principal;

ALTER TABLE rfx.rfx_idempotency_records
    ADD CONSTRAINT fk_rfx_idempotency_integration_principal
    FOREIGN KEY (integration_principal_id)
    REFERENCES rfx.rfx_integration_principals (id);

CREATE INDEX IF NOT EXISTS idx_rfx_idempotency_erp_principal
    ON rfx.rfx_idempotency_records (tenant_id, integration_principal_id, operation)
    WHERE integration_principal_id IS NOT NULL;

ALTER TABLE rfx.rfx_external_object_links
    ADD COLUMN IF NOT EXISTS external_revision VARCHAR(64) NULL;

UPDATE rfx.rfx_external_object_links
SET external_revision = external_version
WHERE external_revision IS NULL;

DO $$
DECLARE
    duplicate_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO duplicate_count
    FROM (
        SELECT
            tenant_id,
            integration_principal_id,
            external_system,
            external_object_type,
            external_object_id
        FROM rfx.rfx_external_object_links
        GROUP BY
            tenant_id,
            integration_principal_id,
            external_system,
            external_object_type,
            external_object_id
        HAVING COUNT(*) > 1
    ) duplicates;

    IF duplicate_count > 0 THEN
        RAISE EXCEPTION 'stable external identity migration blocked: % duplicate stable keys', duplicate_count;
    END IF;
END;
$$;

DROP INDEX IF EXISTS rfx.uq_rfx_external_object_identity;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_external_object_stable_identity
    ON rfx.rfx_external_object_links (
        tenant_id,
        integration_principal_id,
        external_system,
        external_object_type,
        external_object_id
    );

ALTER TABLE rfx.rfx_external_object_links
    DROP CONSTRAINT IF EXISTS fk_rfx_external_object_link_integration_principal;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM rfx.rfx_external_object_links l
        WHERE NOT EXISTS (
            SELECT 1
            FROM rfx.rfx_integration_principals p
            WHERE p.id = l.integration_principal_id
        )
    ) THEN
        RAISE EXCEPTION 'external object link orphan integration_principal_id blocks FK';
    END IF;
END;
$$;

ALTER TABLE rfx.rfx_external_object_links
    ADD CONSTRAINT fk_rfx_external_object_link_integration_principal
    FOREIGN KEY (integration_principal_id)
    REFERENCES rfx.rfx_integration_principals (id);

CREATE TABLE IF NOT EXISTS rfx.rfx_external_object_link_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants (id),
    link_id UUID NOT NULL REFERENCES rfx.rfx_external_object_links (id),
    external_revision VARCHAR(64) NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_external_object_link_revision_nonempty CHECK (
        length(trim(external_revision)) > 0
    ),
    CONSTRAINT chk_rfx_external_object_link_revision_payload_hash CHECK (
        length(trim(payload_hash)) = 64
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_external_object_link_revision
    ON rfx.rfx_external_object_link_revisions (link_id, external_revision);

CREATE INDEX IF NOT EXISTS idx_rfx_external_object_link_revisions_tenant_link
    ON rfx.rfx_external_object_link_revisions (tenant_id, link_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS rfx.rfx_reference_mapping_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NULL REFERENCES core.tenants (id),
    mapping_type VARCHAR(64) NOT NULL,
    version INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_reference_mapping_set_status CHECK (
        status IN ('ACTIVE', 'RETIRED')
    ),
    CONSTRAINT chk_rfx_reference_mapping_set_version_positive CHECK (version > 0),
    CONSTRAINT chk_rfx_reference_mapping_set_type_nonempty CHECK (
        length(trim(mapping_type)) > 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_reference_mapping_sets_tenant_type_version
    ON rfx.rfx_reference_mapping_sets (tenant_id, mapping_type, version)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_reference_mapping_sets_platform_type_version
    ON rfx.rfx_reference_mapping_sets (mapping_type, version)
    WHERE tenant_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_rfx_reference_mapping_sets_lookup
    ON rfx.rfx_reference_mapping_sets (mapping_type, status, version DESC);

DROP TRIGGER IF EXISTS trg_rfx_reference_mapping_sets_updated_at ON rfx.rfx_reference_mapping_sets;

CREATE TRIGGER trg_rfx_reference_mapping_sets_updated_at
    BEFORE UPDATE ON rfx.rfx_reference_mapping_sets
    FOR EACH ROW
    EXECUTE FUNCTION core.set_updated_at();

CREATE TABLE IF NOT EXISTS rfx.rfx_reference_mapping_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants (id),
    mapping_set_id UUID NOT NULL REFERENCES rfx.rfx_reference_mapping_sets (id),
    external_code VARCHAR(128) NOT NULL,
    canonical_code VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_rfx_reference_mapping_entry_external_code_nonempty CHECK (
        length(trim(external_code)) > 0
    ),
    CONSTRAINT chk_rfx_reference_mapping_entry_canonical_code_nonempty CHECK (
        length(trim(canonical_code)) > 0
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_rfx_reference_mapping_entries_set_external_code
    ON rfx.rfx_reference_mapping_entries (mapping_set_id, external_code);

CREATE INDEX IF NOT EXISTS idx_rfx_reference_mapping_entries_tenant_set
    ON rfx.rfx_reference_mapping_entries (tenant_id, mapping_set_id);
