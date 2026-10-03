-- EDO-0.3 I2 signed file attachment foundation.
-- Schema name stays documents. document_files remains the legacy metadata row.

CREATE TABLE documents.document_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    document_version_id UUID,
    file_name VARCHAR(255) NOT NULL,
    media_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 CHAR(64) NOT NULL,
    storage_key TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
    idempotency_key VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finalized_at TIMESTAMPTZ,
    CONSTRAINT uq_document_attachments_id_tenant UNIQUE (id, tenant_id),
    CONSTRAINT uq_document_attachments_storage_key UNIQUE (storage_key),
    CONSTRAINT chk_attachment_status CHECK (status IN ('DRAFT', 'FINALIZED')),
    CONSTRAINT chk_attachment_size CHECK (size_bytes >= 0),
    CONSTRAINT chk_attachment_sha256 CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_attachment_finalized CHECK (
        (status = 'DRAFT' AND finalized_at IS NULL)
        OR (status = 'FINALIZED' AND finalized_at IS NOT NULL)
    ),
    CONSTRAINT fk_attachment_document_tenant
        FOREIGN KEY (document_id, tenant_id)
        REFERENCES documents.documents (id, tenant_id),
    CONSTRAINT fk_attachment_document_version
        FOREIGN KEY (document_id, document_version_id)
        REFERENCES documents.document_versions (document_id, id)
);

CREATE UNIQUE INDEX uq_document_attachments_idempotency
    ON documents.document_attachments (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_document_attachments_tenant_document
    ON documents.document_attachments (tenant_id, document_id);

CREATE INDEX idx_document_attachments_sha256
    ON documents.document_attachments (sha256);

CREATE TABLE documents.attachment_signatures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attachment_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    signature_format VARCHAR(32) NOT NULL,
    signature_reference TEXT NOT NULL,
    certificate_subject TEXT,
    certificate_issuer TEXT,
    certificate_serial VARCHAR(128),
    certificate_thumbprint CHAR(64),
    signing_time TIMESTAMPTZ,
    verification_status VARCHAR(16) NOT NULL DEFAULT 'UNVERIFIED',
    verification_time TIMESTAMPTZ,
    verification_error TEXT,
    idempotency_key VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_attachment_signatures_id_tenant UNIQUE (id, tenant_id),
    CONSTRAINT chk_attachment_signature_status CHECK (
        verification_status IN ('PENDING', 'VALID', 'INVALID', 'UNVERIFIED')
    ),
    CONSTRAINT chk_attachment_signature_thumbprint CHECK (
        certificate_thumbprint IS NULL OR certificate_thumbprint ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT chk_attachment_signature_format CHECK (
        signature_format IN ('CAdES', 'PKCS7', 'XMLDSIG', 'OTHER')
    ),
    CONSTRAINT fk_attachment_signature_attachment_tenant
        FOREIGN KEY (attachment_id, tenant_id)
        REFERENCES documents.document_attachments (id, tenant_id)
);

CREATE UNIQUE INDEX uq_attachment_signatures_idempotency
    ON documents.attachment_signatures (attachment_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_attachment_signatures_attachment
    ON documents.attachment_signatures (tenant_id, attachment_id);

CREATE TABLE documents.signature_verification_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signature_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    verification_status VARCHAR(16) NOT NULL,
    verification_time TIMESTAMPTZ,
    verification_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_signature_history_status CHECK (
        verification_status IN ('PENDING', 'VALID', 'INVALID', 'UNVERIFIED')
    ),
    CONSTRAINT fk_signature_history_signature_tenant
        FOREIGN KEY (signature_id, tenant_id)
        REFERENCES documents.attachment_signatures (id, tenant_id)
);

CREATE INDEX idx_signature_history_signature
    ON documents.signature_verification_history (tenant_id, signature_id, created_at);

CREATE TABLE documents.attachment_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    document_id UUID NOT NULL,
    attachment_id UUID,
    signature_id UUID,
    event_type VARCHAR(64) NOT NULL,
    actor_user_id UUID,
    idempotency_key VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_attachment_audit_event_type CHECK (
        event_type IN (
            'edo.attachment.created',
            'edo.attachment.finalized',
            'edo.signature.attached',
            'edo.signature.verified',
            'edo.signature.verification_failed'
        )
    )
);

CREATE UNIQUE INDEX uq_attachment_audit_replay
    ON documents.attachment_audit_events (tenant_id, event_type, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_attachment_audit_document
    ON documents.attachment_audit_events (tenant_id, document_id, created_at);

CREATE OR REPLACE FUNCTION documents.edo_i2_reject_unverified_claim()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.verification_status IS DISTINCT FROM 'UNVERIFIED' THEN
        RAISE EXCEPTION 'cryptographic verifier is not available';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i2_signatures_unverified
BEFORE INSERT OR UPDATE ON documents.attachment_signatures
FOR EACH ROW EXECUTE FUNCTION documents.edo_i2_reject_unverified_claim();

CREATE TRIGGER trg_edo_i2_history_unverified
BEFORE INSERT OR UPDATE ON documents.signature_verification_history
FOR EACH ROW EXECUTE FUNCTION documents.edo_i2_reject_unverified_claim();

CREATE OR REPLACE FUNCTION documents.edo_i2_reject_history_rewrite()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'signature verification history is append-only';
END;
$$;

CREATE TRIGGER trg_edo_i2_history_append
BEFORE UPDATE OR DELETE ON documents.signature_verification_history
FOR EACH ROW EXECUTE FUNCTION documents.edo_i2_reject_history_rewrite();

CREATE OR REPLACE FUNCTION documents.edo_i2_reject_finalized_attachment_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status = 'FINALIZED' THEN
            RAISE EXCEPTION 'finalized attachment is immutable';
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.status = 'FINALIZED' AND (
        NEW.sha256 IS DISTINCT FROM OLD.sha256
        OR NEW.storage_key IS DISTINCT FROM OLD.storage_key
        OR NEW.size_bytes IS DISTINCT FROM OLD.size_bytes
        OR NEW.file_name IS DISTINCT FROM OLD.file_name
        OR NEW.media_type IS DISTINCT FROM OLD.media_type
        OR NEW.document_id IS DISTINCT FROM OLD.document_id
        OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
        OR NEW.document_version_id IS DISTINCT FROM OLD.document_version_id
        OR NEW.status IS DISTINCT FROM OLD.status
    ) THEN
        RAISE EXCEPTION 'finalized attachment is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i2_attachment_immutable
BEFORE UPDATE OR DELETE ON documents.document_attachments
FOR EACH ROW EXECUTE FUNCTION documents.edo_i2_reject_finalized_attachment_change();

CREATE OR REPLACE FUNCTION documents.edo_i2_reject_signature_rewrite()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'attachment signature metadata is immutable';
    END IF;
    IF NEW.attachment_id IS DISTINCT FROM OLD.attachment_id
        OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
        OR NEW.signature_format IS DISTINCT FROM OLD.signature_format
        OR NEW.signature_reference IS DISTINCT FROM OLD.signature_reference
        OR NEW.certificate_subject IS DISTINCT FROM OLD.certificate_subject
        OR NEW.certificate_issuer IS DISTINCT FROM OLD.certificate_issuer
        OR NEW.certificate_serial IS DISTINCT FROM OLD.certificate_serial
        OR NEW.certificate_thumbprint IS DISTINCT FROM OLD.certificate_thumbprint
        OR NEW.signing_time IS DISTINCT FROM OLD.signing_time
        OR NEW.verification_status IS DISTINCT FROM OLD.verification_status
        OR NEW.verification_time IS DISTINCT FROM OLD.verification_time
        OR NEW.verification_error IS DISTINCT FROM OLD.verification_error
    THEN
        RAISE EXCEPTION 'attachment signature metadata is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_edo_i2_signature_immutable
BEFORE UPDATE OR DELETE ON documents.attachment_signatures
FOR EACH ROW EXECUTE FUNCTION documents.edo_i2_reject_signature_rewrite();
