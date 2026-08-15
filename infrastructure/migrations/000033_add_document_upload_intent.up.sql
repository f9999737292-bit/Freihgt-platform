CREATE TABLE IF NOT EXISTS documents.document_upload_intent (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    document_id UUID NOT NULL REFERENCES documents.documents(id),
    shipment_id UUID NOT NULL,
    driver_id UUID NOT NULL,
    object_key TEXT NOT NULL,
    upload_token_hash TEXT NOT NULL,
    mime_type VARCHAR(128) NOT NULL,
    max_bytes BIGINT NOT NULL,
    file_name VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(128),
    checksum_sha256 VARCHAR(64),
    byte_size BIGINT,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_document_upload_intent_idempotency UNIQUE (tenant_id, driver_id, idempotency_key),
    CONSTRAINT chk_document_upload_intent_status CHECK (status IN ('pending', 'uploaded', 'completed', 'expired'))
);

CREATE INDEX idx_document_upload_intent_document ON documents.document_upload_intent (tenant_id, document_id);
