-- EDO-POD-DELIVERY-ACTION-EVIDENCE-BINDING-0.1B document runtime.
-- Dedicated action-bound POD upload intent and immutable delivery-action evidence.
-- No foreign key into transport. No historical shipment-only POD backfill.
-- document_tenant_id is the execution operating tenant and equals operating_tenant_id.
-- shipment_tenant_id stays the canonical shipment owner and may differ.

CREATE TABLE documents.pod_delivery_action_upload_intent (
    id uuid PRIMARY KEY,
    document_tenant_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    execution_action_id uuid NOT NULL,
    execution_stop_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    owner_company_id uuid NOT NULL,
    author_driver_id uuid NOT NULL,
    author_company_id uuid NOT NULL,
    document_id uuid NOT NULL UNIQUE REFERENCES documents.documents (id),
    document_version_id uuid NOT NULL REFERENCES documents.document_versions (id),
    object_key text NOT NULL,
    upload_token_hash text NOT NULL,
    mime_type varchar(128) NOT NULL,
    max_bytes bigint NOT NULL,
    file_name varchar(255),
    status varchar(32) NOT NULL DEFAULT 'pending',
    idempotency_key varchar(128) NOT NULL,
    checksum_sha256 varchar(64),
    byte_size bigint,
    expires_at timestamptz NOT NULL,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pod_delivery_action_upload_intent_tenant_eq_chk
        CHECK (document_tenant_id = operating_tenant_id),
    CONSTRAINT pod_delivery_action_upload_intent_key_len_chk
        CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    CONSTRAINT pod_delivery_action_upload_intent_status_chk
        CHECK (status IN ('pending', 'uploaded', 'completed')),
    CONSTRAINT pod_delivery_action_upload_intent_idempotency_uq
        UNIQUE (document_tenant_id, idempotency_key),
    CONSTRAINT pod_delivery_action_upload_intent_action_key_uq
        UNIQUE (document_tenant_id, execution_action_id, idempotency_key)
);

CREATE INDEX pod_delivery_action_upload_intent_action_idx
    ON documents.pod_delivery_action_upload_intent (operating_tenant_id, execution_id, execution_action_id);

CREATE TABLE documents.pod_delivery_action_evidence (
    id uuid PRIMARY KEY,
    upload_intent_id uuid NOT NULL UNIQUE REFERENCES documents.pod_delivery_action_upload_intent (id),
    document_tenant_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    execution_action_id uuid NOT NULL,
    execution_stop_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    cargo_id uuid NOT NULL,
    owner_company_id uuid NOT NULL,
    author_driver_id uuid NOT NULL,
    author_company_id uuid NOT NULL,
    document_id uuid NOT NULL UNIQUE REFERENCES documents.documents (id),
    document_version_id uuid NOT NULL REFERENCES documents.document_versions (id),
    document_file_id uuid NOT NULL UNIQUE REFERENCES documents.document_files (id),
    idempotency_key varchar(128) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pod_delivery_action_evidence_tenant_eq_chk
        CHECK (document_tenant_id = operating_tenant_id),
    CONSTRAINT pod_delivery_action_evidence_key_len_chk
        CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    CONSTRAINT pod_delivery_action_evidence_idempotency_uq
        UNIQUE (document_tenant_id, execution_action_id, idempotency_key)
);

CREATE INDEX pod_delivery_action_evidence_subject_idx
    ON documents.pod_delivery_action_evidence (operating_tenant_id, execution_id, execution_action_id);

CREATE INDEX pod_delivery_action_evidence_document_idx
    ON documents.pod_delivery_action_evidence (document_tenant_id, document_id);

CREATE FUNCTION documents.reject_pod_delivery_action_intent_pin_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.document_tenant_id IS DISTINCT FROM OLD.document_tenant_id
        OR NEW.operating_tenant_id IS DISTINCT FROM OLD.operating_tenant_id
        OR NEW.execution_id IS DISTINCT FROM OLD.execution_id
        OR NEW.execution_action_id IS DISTINCT FROM OLD.execution_action_id
        OR NEW.execution_stop_id IS DISTINCT FROM OLD.execution_stop_id
        OR NEW.shipment_id IS DISTINCT FROM OLD.shipment_id
        OR NEW.shipment_tenant_id IS DISTINCT FROM OLD.shipment_tenant_id
        OR NEW.cargo_id IS DISTINCT FROM OLD.cargo_id
        OR NEW.owner_company_id IS DISTINCT FROM OLD.owner_company_id
        OR NEW.author_driver_id IS DISTINCT FROM OLD.author_driver_id
        OR NEW.author_company_id IS DISTINCT FROM OLD.author_company_id
        OR NEW.document_id IS DISTINCT FROM OLD.document_id
        OR NEW.document_version_id IS DISTINCT FROM OLD.document_version_id
        OR NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key
        OR NEW.object_key IS DISTINCT FROM OLD.object_key
    THEN
        RAISE EXCEPTION 'POD_ACTION_INTENT_PIN_IMMUTABLE'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_pod_delivery_action_intent_pin
    BEFORE UPDATE ON documents.pod_delivery_action_upload_intent
    FOR EACH ROW
    EXECUTE FUNCTION documents.reject_pod_delivery_action_intent_pin_update();

CREATE FUNCTION documents.reject_pod_delivery_action_evidence_identity_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'POD_ACTION_EVIDENCE_IMMUTABLE'
        USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER trg_pod_delivery_action_evidence_identity
    BEFORE UPDATE ON documents.pod_delivery_action_evidence
    FOR EACH ROW
    EXECUTE FUNCTION documents.reject_pod_delivery_action_evidence_identity_update();
