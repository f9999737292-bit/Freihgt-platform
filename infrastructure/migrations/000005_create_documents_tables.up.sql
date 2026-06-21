CREATE TABLE documents.documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    document_number VARCHAR(100) NOT NULL,
    document_type VARCHAR(50) NOT NULL,
    document_status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    owner_company_id UUID NOT NULL,
    related_entity_type VARCHAR(100),
    related_entity_id UUID,
    legal_language VARCHAR(10) NOT NULL DEFAULT 'ru-RU',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_document_number UNIQUE (tenant_id, document_number),
    CONSTRAINT chk_document_type CHECK (
        document_type IN (
            'ETRN','EPD','WAYBILL','POD','DISCREPANCY_ACT','CLAIM',
            'INVOICE','VAT_INVOICE','ACT','UPD','ECMR'
        )
    ),
    CONSTRAINT chk_document_status CHECK (
        document_status IN (
            'DRAFT','READY_FOR_SIGNING','SIGNING_IN_PROGRESS','SIGNED',
            'SENT_TO_OPERATOR','ACCEPTED','REJECTED','ARCHIVED','CANCELLED'
        )
    )
);

CREATE INDEX idx_documents_tenant_id ON documents.documents(tenant_id);
CREATE INDEX idx_documents_owner_company_id ON documents.documents(owner_company_id);
CREATE INDEX idx_documents_type ON documents.documents(document_type);
CREATE INDEX idx_documents_status ON documents.documents(document_status);
CREATE INDEX idx_documents_related_entity ON documents.documents(related_entity_type, related_entity_id);

CREATE TABLE documents.document_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents.documents(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    payload_json JSONB,
    payload_xml_path TEXT,
    pdf_file_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT uq_document_version UNIQUE (document_id, version_number)
);

CREATE INDEX idx_document_versions_document_id ON documents.document_versions(document_id);

CREATE TABLE documents.document_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents.documents(id) ON DELETE CASCADE,
    document_version_id UUID REFERENCES documents.document_versions(id) ON DELETE SET NULL,
    file_type VARCHAR(50) NOT NULL,
    storage_provider VARCHAR(50) NOT NULL DEFAULT 'S3',
    bucket_name VARCHAR(255),
    object_key TEXT NOT NULL,
    file_name VARCHAR(500),
    mime_type VARCHAR(255),
    file_size_bytes BIGINT,
    checksum_sha256 VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID
);

CREATE INDEX idx_document_files_document_id ON documents.document_files(document_id);
CREATE INDEX idx_document_files_version_id ON documents.document_files(document_version_id);
CREATE INDEX idx_document_files_type ON documents.document_files(file_type);

CREATE TABLE documents.signing_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    document_id UUID NOT NULL REFERENCES documents.documents(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'CREATED',
    required_signers_count INTEGER NOT NULL DEFAULT 1,
    completed_signers_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    expires_at TIMESTAMPTZ,
    CONSTRAINT chk_signing_session_status CHECK (
        status IN ('CREATED','IN_PROGRESS','COMPLETED','EXPIRED','CANCELLED','FAILED')
    )
);

CREATE INDEX idx_signing_sessions_tenant_id ON documents.signing_sessions(tenant_id);
CREATE INDEX idx_signing_sessions_document_id ON documents.signing_sessions(document_id);
CREATE INDEX idx_signing_sessions_status ON documents.signing_sessions(status);

CREATE TABLE documents.signatures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    signing_session_id UUID NOT NULL REFERENCES documents.signing_sessions(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents.documents(id) ON DELETE CASCADE,
    signer_user_id UUID,
    signer_company_id UUID,
    signature_type VARCHAR(50) NOT NULL,
    signature_payload_path TEXT,
    certificate_fingerprint VARCHAR(255),
    signed_at TIMESTAMPTZ,
    verification_status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_signature_verification_status CHECK (
        verification_status IN ('PENDING','VALID','INVALID','EXPIRED','REVOKED','FAILED')
    )
);

CREATE INDEX idx_signatures_tenant_id ON documents.signatures(tenant_id);
CREATE INDEX idx_signatures_session_id ON documents.signatures(signing_session_id);
CREATE INDEX idx_signatures_document_id ON documents.signatures(document_id);
CREATE INDEX idx_signatures_signer_user_id ON documents.signatures(signer_user_id);
CREATE INDEX idx_signatures_signer_company_id ON documents.signatures(signer_company_id);
