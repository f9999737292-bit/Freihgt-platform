-- EDO-0.3 I1 schema foundation. Additive documents schema only.
-- No legacy signature revision is inferred.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM documents.signatures s
        JOIN documents.documents d ON d.id = s.document_id
        WHERE s.tenant_id IS DISTINCT FROM d.tenant_id
    ) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 cross-tenant signature/document mismatch; refusing inferred repair';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM documents.signatures s
        JOIN documents.signing_sessions ss ON ss.id = s.signing_session_id
        WHERE s.tenant_id IS DISTINCT FROM ss.tenant_id
    ) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 cross-tenant signature/session mismatch; refusing inferred repair';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM documents.signing_sessions ss
        JOIN documents.documents d ON d.id = ss.document_id
        WHERE ss.tenant_id IS DISTINCT FROM d.tenant_id
    ) THEN
        RAISE EXCEPTION 'EDO-0.3 I1 cross-tenant session/document mismatch; refusing inferred repair';
    END IF;
END $$;

ALTER TABLE documents.documents
    ADD CONSTRAINT uq_documents_id_tenant UNIQUE (id, tenant_id);

ALTER TABLE documents.signing_sessions
    ADD CONSTRAINT uq_signing_sessions_id_tenant UNIQUE (id, tenant_id);

ALTER TABLE documents.signatures
    ADD CONSTRAINT uq_signatures_id_tenant UNIQUE (id, tenant_id);

ALTER TABLE documents.document_versions
    ADD CONSTRAINT uq_document_versions_document_id_id UNIQUE (document_id, id);

ALTER TABLE documents.signatures
    ADD COLUMN document_version_id UUID,
    ADD COLUMN content_digest_algorithm VARCHAR(32),
    ADD COLUMN content_digest_value VARCHAR(64);

ALTER TABLE documents.signatures
    ADD CONSTRAINT chk_signatures_revision_binding CHECK (
        (
            document_version_id IS NULL
            AND content_digest_algorithm IS NULL
            AND content_digest_value IS NULL
        )
        OR
        (
            document_version_id IS NOT NULL
            AND content_digest_algorithm IS NOT NULL
            AND content_digest_value IS NOT NULL
        )
    ),
    ADD CONSTRAINT chk_signatures_digest_algorithm CHECK (
        content_digest_algorithm IS NULL OR content_digest_algorithm = 'SHA-256'
    ),
    ADD CONSTRAINT chk_signatures_digest_value CHECK (
        content_digest_value IS NULL OR content_digest_value ~ '^[0-9a-f]{64}$'
    ),
    ADD CONSTRAINT uq_signatures_id_version UNIQUE (id, document_version_id),
    ADD CONSTRAINT fk_signatures_document_tenant
        FOREIGN KEY (document_id, tenant_id)
        REFERENCES documents.documents (id, tenant_id),
    ADD CONSTRAINT fk_signatures_session_tenant
        FOREIGN KEY (signing_session_id, tenant_id)
        REFERENCES documents.signing_sessions (id, tenant_id),
    ADD CONSTRAINT fk_signatures_document_version
        FOREIGN KEY (document_id, document_version_id)
        REFERENCES documents.document_versions (document_id, id)
        ON DELETE RESTRICT;

ALTER TABLE documents.signing_sessions
    ADD CONSTRAINT fk_signing_sessions_document_tenant
        FOREIGN KEY (document_id, tenant_id)
        REFERENCES documents.documents (id, tenant_id);

CREATE TABLE documents.document_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    assembling_company_id UUID NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sealed_at TIMESTAMPTZ,
    sealed_by UUID,
    idempotency_key VARCHAR(128),
    request_fingerprint CHAR(64),
    seal_idempotency_key VARCHAR(128),
    seal_request_fingerprint CHAR(64),
    CONSTRAINT uq_document_packages_id_tenant UNIQUE (id, tenant_id),
    CONSTRAINT chk_document_packages_status CHECK (
        (status = 'OPEN' AND sealed_at IS NULL)
        OR (status = 'SEALED' AND sealed_at IS NOT NULL)
    ),
    CONSTRAINT chk_document_packages_request_fingerprint CHECK (
        request_fingerprint IS NULL OR request_fingerprint ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT chk_document_packages_seal_fingerprint CHECK (
        seal_request_fingerprint IS NULL OR seal_request_fingerprint ~ '^[0-9a-f]{64}$'
    )
);

CREATE UNIQUE INDEX uq_document_packages_idempotency
    ON documents.document_packages (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX uq_document_packages_seal_idempotency
    ON documents.document_packages (tenant_id, seal_idempotency_key)
    WHERE seal_idempotency_key IS NOT NULL;

CREATE TABLE documents.document_package_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id UUID NOT NULL,
    document_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT uq_package_members_package_document UNIQUE (package_id, document_id),
    CONSTRAINT fk_package_members_package_tenant
        FOREIGN KEY (package_id, tenant_id)
        REFERENCES documents.document_packages (id, tenant_id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_package_members_document_tenant
        FOREIGN KEY (document_id, tenant_id)
        REFERENCES documents.documents (id, tenant_id)
        ON DELETE RESTRICT
);

CREATE TABLE documents.document_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    source_document_id UUID NOT NULL,
    target_document_id UUID NOT NULL,
    relationship_type VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    idempotency_key VARCHAR(128),
    request_fingerprint CHAR(64),
    CONSTRAINT chk_document_relationships_type CHECK (
        relationship_type IN ('CORRECTS', 'REPLACES', 'RELATED_TO')
    ),
    CONSTRAINT chk_document_relationships_distinct CHECK (
        source_document_id <> target_document_id
    ),
    CONSTRAINT chk_document_relationships_fingerprint CHECK (
        request_fingerprint IS NULL OR request_fingerprint ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT fk_relationships_source_tenant
        FOREIGN KEY (source_document_id, tenant_id)
        REFERENCES documents.documents (id, tenant_id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_relationships_target_tenant
        FOREIGN KEY (target_document_id, tenant_id)
        REFERENCES documents.documents (id, tenant_id)
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX uq_document_relationships_idempotency
    ON documents.document_relationships (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE documents.certificate_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    signature_id UUID NOT NULL,
    document_version_id UUID NOT NULL,
    certificate_fingerprint VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT uq_certificate_evidence_signature UNIQUE (signature_id),
    CONSTRAINT fk_certificate_evidence_signature_tenant
        FOREIGN KEY (signature_id, tenant_id)
        REFERENCES documents.signatures (id, tenant_id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_certificate_evidence_signature_version
        FOREIGN KEY (signature_id, document_version_id)
        REFERENCES documents.signatures (id, document_version_id)
        ON DELETE RESTRICT
);

CREATE TABLE documents.signature_verification_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    signature_id UUID NOT NULL,
    document_version_id UUID,
    verification_status VARCHAR(50) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    recorded_by UUID,
    CONSTRAINT chk_signature_verification_status CHECK (
        verification_status IN ('PENDING', 'VALID', 'INVALID', 'EXPIRED', 'REVOKED', 'FAILED')
    ),
    CONSTRAINT fk_signature_verification_signature_tenant
        FOREIGN KEY (signature_id, tenant_id)
        REFERENCES documents.signatures (id, tenant_id)
        ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION documents.edo_i1_document_is_signed_class(doc_id UUID)
RETURNS boolean
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(
        (
            SELECT document_status IN ('SIGNED', 'SENT_TO_OPERATOR', 'ACCEPTED', 'ARCHIVED')
            FROM documents.documents
            WHERE id = doc_id
        ),
        false
    );
$$;

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_signed_document_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF documents.edo_i1_document_is_signed_class(OLD.id) THEN
        RAISE EXCEPTION 'signed document delete is forbidden';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_edo_i1_documents_delete
BEFORE DELETE ON documents.documents
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_signed_document_delete();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_signed_document_soft_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.document_status IN ('SIGNED', 'SENT_TO_OPERATOR', 'ACCEPTED', 'ARCHIVED')
       AND OLD.deleted_at IS NULL
       AND NEW.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION 'signed document soft delete is forbidden';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i1_documents_soft_delete
BEFORE UPDATE ON documents.documents
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_signed_document_soft_delete();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_signed_version_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF documents.edo_i1_document_is_signed_class(OLD.document_id)
       OR EXISTS (
            SELECT 1 FROM documents.signatures
            WHERE document_version_id = OLD.id
       ) THEN
        RAISE EXCEPTION 'signed revision delete is forbidden';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_edo_i1_versions_delete
BEFORE DELETE ON documents.document_versions
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_signed_version_delete();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_signed_payload_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.payload_json IS NOT DISTINCT FROM OLD.payload_json
       AND NEW.payload_xml_path IS NOT DISTINCT FROM OLD.payload_xml_path
       AND NEW.pdf_file_path IS NOT DISTINCT FROM OLD.pdf_file_path THEN
        RETURN NEW;
    END IF;
    IF documents.edo_i1_document_is_signed_class(OLD.document_id)
       OR EXISTS (
            SELECT 1 FROM documents.signatures
            WHERE document_version_id = OLD.id
       ) THEN
        RAISE EXCEPTION 'signed revision payload update is forbidden';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i1_versions_payload
BEFORE UPDATE ON documents.document_versions
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_signed_payload_update();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_signed_file_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF documents.edo_i1_document_is_signed_class(OLD.document_id) THEN
        RAISE EXCEPTION 'signed document file delete is forbidden';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_edo_i1_files_delete
BEFORE DELETE ON documents.document_files
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_signed_file_delete();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_signed_session_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF documents.edo_i1_document_is_signed_class(OLD.document_id) THEN
        RAISE EXCEPTION 'signed signing session delete is forbidden';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_edo_i1_sessions_delete
BEFORE DELETE ON documents.signing_sessions
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_signed_session_delete();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_signed_signature_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.document_version_id IS NOT NULL
       OR documents.edo_i1_document_is_signed_class(OLD.document_id) THEN
        RAISE EXCEPTION 'signed signature delete is forbidden';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_edo_i1_signatures_delete
BEFORE DELETE ON documents.signatures
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_signed_signature_delete();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_bound_signature_rewrite()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.document_version_id IS NOT NULL AND (
        NEW.document_version_id IS DISTINCT FROM OLD.document_version_id
        OR NEW.content_digest_algorithm IS DISTINCT FROM OLD.content_digest_algorithm
        OR NEW.content_digest_value IS DISTINCT FROM OLD.content_digest_value
        OR NEW.certificate_fingerprint IS DISTINCT FROM OLD.certificate_fingerprint
        OR NEW.verification_status IS DISTINCT FROM OLD.verification_status
    ) THEN
        RAISE EXCEPTION 'bound signature evidence rewrite is forbidden';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i1_signatures_binding
BEFORE UPDATE ON documents.signatures
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_bound_signature_rewrite();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_sealed_membership()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    pkg_status VARCHAR(16);
BEGIN
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        SELECT status INTO pkg_status
        FROM documents.document_packages
        WHERE id = NEW.package_id;
        IF pkg_status = 'SEALED' THEN
            RAISE EXCEPTION 'sealed package membership is immutable';
        END IF;
    END IF;
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        SELECT status INTO pkg_status
        FROM documents.document_packages
        WHERE id = OLD.package_id;
        IF pkg_status = 'SEALED' THEN
            RAISE EXCEPTION 'sealed package membership is immutable';
        END IF;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i1_members_seal
BEFORE INSERT OR UPDATE OR DELETE ON documents.document_package_members
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_sealed_membership();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_relationship_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'document relationship is append-only';
END;
$$;

CREATE TRIGGER trg_edo_i1_relationships_append
BEFORE UPDATE OR DELETE ON documents.document_relationships
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_relationship_mutation();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_certificate_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'certificate evidence is append-only';
END;
$$;

CREATE TRIGGER trg_edo_i1_certificate_append
BEFORE UPDATE OR DELETE ON documents.certificate_evidence
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_certificate_mutation();

CREATE OR REPLACE FUNCTION documents.edo_i1_reject_verification_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'signature verification history is append-only';
END;
$$;

CREATE TRIGGER trg_edo_i1_verification_append
BEFORE UPDATE OR DELETE ON documents.signature_verification_records
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_reject_verification_mutation();

CREATE OR REPLACE FUNCTION documents.edo_i1_verification_matches_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    bound UUID;
BEGIN
    SELECT document_version_id INTO bound
    FROM documents.signatures
    WHERE id = NEW.signature_id AND tenant_id = NEW.tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'verification record signature is not in the same tenant';
    END IF;
    IF bound IS NULL AND NEW.document_version_id IS NOT NULL THEN
        RAISE EXCEPTION 'unbound signature verification cannot name a revision';
    END IF;
    IF bound IS NOT NULL AND NEW.document_version_id IS DISTINCT FROM bound THEN
        RAISE EXCEPTION 'verification record revision must match the signature binding';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i1_verification_insert
BEFORE INSERT ON documents.signature_verification_records
FOR EACH ROW EXECUTE FUNCTION documents.edo_i1_verification_matches_binding();
