-- EDO-0.3 I4B detached signature blob metadata and append-only verification evidence.
-- The signature bytes stay in object storage. These tables do not store the blob.
-- I4B evidence is PENDING / VERIFIER_UNAVAILABLE only. A later migration may widen that gate.

CREATE TABLE documents.attachment_signature_blobs (
    signature_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    object_key TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 CHAR(64) NOT NULL,
    media_type VARCHAR(255) NOT NULL,
    profile VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (signature_id, tenant_id),
    CONSTRAINT uq_attachment_signature_blobs_object_key UNIQUE (object_key),
    CONSTRAINT chk_attachment_signature_blob_size CHECK (size_bytes > 0 AND size_bytes <= 1048576),
    CONSTRAINT chk_attachment_signature_blob_sha256 CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_attachment_signature_blob_profile CHECK (profile = 'CAdES-BES'),
    -- profile is the accepted upload profile, not a cryptographic observation of CAdES-BES.
    CONSTRAINT fk_attachment_signature_blob_signature
        FOREIGN KEY (signature_id, tenant_id)
        REFERENCES documents.attachment_signatures (id, tenant_id)
);

CREATE TABLE documents.signature_verification_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signature_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    verification_status VARCHAR(16) NOT NULL,
    reason_code VARCHAR(64) NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL,
    verifier_version VARCHAR(64) NOT NULL,
    policy_id VARCHAR(64) NOT NULL,
    policy_version VARCHAR(32) NOT NULL,
    certificate_subject TEXT,
    certificate_issuer TEXT,
    certificate_serial VARCHAR(128),
    certificate_thumbprint CHAR(64),
    certificate_valid_from TIMESTAMPTZ,
    certificate_valid_to TIMESTAMPTZ,
    chain_fingerprint CHAR(64),
    chain_status VARCHAR(32),
    revocation_status VARCHAR(32),
    revocation_evidence_time TIMESTAMPTZ,
    timestamp_status VARCHAR(32),
    timestamp_token_object_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_signature_evidence_i4b_status CHECK (verification_status = 'PENDING'),
    CONSTRAINT chk_signature_evidence_i4b_reason CHECK (reason_code = 'VERIFIER_UNAVAILABLE'),
    CONSTRAINT chk_signature_evidence_thumbprint CHECK (
        certificate_thumbprint IS NULL OR certificate_thumbprint ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT chk_signature_evidence_chain_fingerprint CHECK (
        chain_fingerprint IS NULL OR chain_fingerprint ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT fk_signature_evidence_signature
        FOREIGN KEY (signature_id, tenant_id)
        REFERENCES documents.attachment_signatures (id, tenant_id),
    CONSTRAINT fk_signature_evidence_blob
        FOREIGN KEY (signature_id, tenant_id)
        REFERENCES documents.attachment_signature_blobs (signature_id, tenant_id)
);

COMMENT ON COLUMN documents.attachment_signature_blobs.profile IS
    'Accepted upload profile declared by the binary path. Not server-verified CAdES evidence and not a basis for VALID.';

CREATE INDEX idx_signature_evidence_policy
    ON documents.signature_verification_evidence (tenant_id, signature_id, policy_id, policy_version, created_at);

CREATE OR REPLACE FUNCTION documents.edo_i4b_reject_blob_rewrite()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'attachment signature blob metadata is immutable';
END;
$$;

CREATE TRIGGER trg_edo_i4b_blob_immutable
BEFORE UPDATE OR DELETE ON documents.attachment_signature_blobs
FOR EACH ROW EXECUTE FUNCTION documents.edo_i4b_reject_blob_rewrite();

CREATE OR REPLACE FUNCTION documents.edo_i4b_reject_evidence_rewrite()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'signature verification evidence is append-only';
END;
$$;

CREATE TRIGGER trg_edo_i4b_evidence_append
BEFORE UPDATE OR DELETE ON documents.signature_verification_evidence
FOR EACH ROW EXECUTE FUNCTION documents.edo_i4b_reject_evidence_rewrite();
