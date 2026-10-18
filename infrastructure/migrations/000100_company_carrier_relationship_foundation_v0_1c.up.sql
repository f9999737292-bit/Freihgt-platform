-- Company-owned carrier relationship foundation.
-- New company schema only. Existing historical rows are not updated.

CREATE SCHEMA IF NOT EXISTS company;

CREATE TABLE company.carrier_relationship_requests (
    relationship_request_id UUID PRIMARY KEY,
    source_type VARCHAR(64) NOT NULL,
    source_id UUID NOT NULL,
    source_tenant_id UUID NOT NULL REFERENCES core.tenants(id),
    source_version_or_fingerprint TEXT NOT NULL,
    shipper_carrier_company_id UUID NOT NULL REFERENCES core.companies(id),
    requesting_company_id UUID NOT NULL REFERENCES core.companies(id),
    requesting_user_id UUID NOT NULL REFERENCES core.users(id),
    acceptance_token_hash BYTEA NOT NULL,
    token_created_at TIMESTAMPTZ NOT NULL,
    token_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    created_by UUID NOT NULL,
    CONSTRAINT chk_crr_source_type CHECK (source_type IN ('SHIPMENT', 'RFX_AWARD', 'AWARD_TRANSPORT_ORDER_LINK')),
    CONSTRAINT chk_crr_token_hash_len CHECK (octet_length(acceptance_token_hash) = 32),
    CONSTRAINT chk_crr_fingerprint_present CHECK (length(btrim(source_version_or_fingerprint)) > 0),
    CONSTRAINT chk_crr_token_ttl CHECK (token_expires_at = token_created_at + interval '72 hours')
);

CREATE TABLE company.carrier_relationship_request_events (
    event_id UUID PRIMARY KEY,
    relationship_request_id UUID NOT NULL REFERENCES company.carrier_relationship_requests(relationship_request_id),
    request_version INTEGER NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    request_status VARCHAR(32) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    actor_user_id UUID NOT NULL,
    cancelled_at TIMESTAMPTZ,
    cancelled_by UUID,
    cancel_reason TEXT,
    accepted_at TIMESTAMPTZ,
    accepted_by UUID,
    accepted_operating_tenant_id UUID,
    accepted_operating_carrier_company_id UUID,
    relationship_id UUID,
    relationship_version INTEGER,
    CONSTRAINT chk_crr_event_type CHECK (event_type IN ('CREATED', 'ACCEPTED', 'EXPIRED', 'CANCELLED')),
    CONSTRAINT chk_crr_event_status CHECK (request_status IN ('PENDING', 'ACCEPTED', 'EXPIRED', 'CANCELLED')),
    CONSTRAINT chk_crr_event_version CHECK (request_version >= 1),
    CONSTRAINT uq_crr_event_version UNIQUE (relationship_request_id, request_version)
);

CREATE UNIQUE INDEX uq_crr_event_accepted
    ON company.carrier_relationship_request_events (relationship_request_id)
    WHERE event_type = 'ACCEPTED';

-- source_type, source_id, and shipper_carrier_company_id are immutable copies of the
-- request root so one PENDING request per source and carrier can be enforced by a
-- partial unique index. A unique index cannot join the root.
CREATE TABLE company.carrier_relationship_request_heads (
    relationship_request_id UUID PRIMARY KEY REFERENCES company.carrier_relationship_requests(relationship_request_id),
    source_type VARCHAR(64) NOT NULL,
    source_id UUID NOT NULL,
    shipper_carrier_company_id UUID NOT NULL,
    current_event_id UUID NOT NULL REFERENCES company.carrier_relationship_request_events(event_id),
    current_version INTEGER NOT NULL,
    request_status VARCHAR(32) NOT NULL,
    CONSTRAINT chk_crr_head_status CHECK (request_status IN ('PENDING', 'ACCEPTED', 'EXPIRED', 'CANCELLED')),
    CONSTRAINT chk_crr_head_version CHECK (current_version >= 1)
);

CREATE UNIQUE INDEX uq_crr_one_pending_source_carrier
    ON company.carrier_relationship_request_heads (source_type, source_id, shipper_carrier_company_id)
    WHERE request_status = 'PENDING';

CREATE UNIQUE INDEX uq_crr_acceptance_token_hash
    ON company.carrier_relationship_requests (acceptance_token_hash);

CREATE TABLE company.carrier_relationship_request_idempotency (
    caller_service VARCHAR(64) NOT NULL,
    source_tenant_id UUID NOT NULL,
    requesting_company_id UUID NOT NULL,
    idempotency_key VARCHAR(200) NOT NULL,
    request_hash BYTEA NOT NULL,
    relationship_request_id UUID NOT NULL REFERENCES company.carrier_relationship_requests(relationship_request_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (caller_service, source_tenant_id, requesting_company_id, idempotency_key),
    CONSTRAINT chk_crr_idempotency_key CHECK (length(btrim(idempotency_key)) > 0),
    CONSTRAINT chk_crr_request_hash_len CHECK (octet_length(request_hash) = 32)
);

CREATE TABLE company.carrier_relationships (
    relationship_id UUID PRIMARY KEY,
    shipper_tenant_id UUID NOT NULL REFERENCES core.tenants(id),
    shipper_carrier_company_id UUID NOT NULL REFERENCES core.companies(id),
    operating_tenant_id UUID NOT NULL REFERENCES core.tenants(id),
    operating_carrier_company_id UUID NOT NULL REFERENCES core.companies(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_cr_business_key UNIQUE (shipper_tenant_id, shipper_carrier_company_id, operating_tenant_id)
);

CREATE TABLE company.carrier_relationship_versions (
    relationship_version_id UUID PRIMARY KEY,
    relationship_id UUID NOT NULL REFERENCES company.carrier_relationships(relationship_id),
    relationship_version INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL,
    supersedes_relationship_version_id UUID,
    source_relationship_request_id UUID NOT NULL REFERENCES company.carrier_relationship_requests(relationship_request_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    created_by UUID NOT NULL,
    CONSTRAINT chk_cr_version_status CHECK (status IN ('ACTIVE', 'REVOKED')),
    CONSTRAINT chk_cr_version_number CHECK (relationship_version >= 1),
    CONSTRAINT uq_cr_version UNIQUE (relationship_id, relationship_version)
);

CREATE TABLE company.carrier_relationship_heads (
    relationship_id UUID PRIMARY KEY REFERENCES company.carrier_relationships(relationship_id),
    current_version INTEGER NOT NULL,
    current_version_id UUID NOT NULL REFERENCES company.carrier_relationship_versions(relationship_version_id),
    status VARCHAR(32) NOT NULL,
    CONSTRAINT chk_cr_head_status CHECK (status IN ('ACTIVE', 'REVOKED')),
    CONSTRAINT uq_cr_head_version UNIQUE (current_version_id)
);

CREATE TABLE company.carrier_relationship_acceptance_idempotency (
    actor_tenant_id UUID NOT NULL,
    idempotency_key VARCHAR(200) NOT NULL,
    operating_company_id UUID NOT NULL,
    relationship_request_id UUID NOT NULL REFERENCES company.carrier_relationship_requests(relationship_request_id),
    body_hash BYTEA NOT NULL,
    relationship_id UUID NOT NULL,
    relationship_version INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (actor_tenant_id, idempotency_key),
    CONSTRAINT chk_cra_idempotency_key CHECK (length(btrim(idempotency_key)) > 0),
    CONSTRAINT chk_cra_body_hash_len CHECK (octet_length(body_hash) = 32)
);

CREATE TABLE company.carrier_relationship_audit (
    audit_id UUID PRIMARY KEY,
    actor_user_id UUID NOT NULL,
    actor_tenant_id UUID NOT NULL,
    selected_company_id UUID NOT NULL,
    relationship_request_id UUID,
    relationship_id UUID,
    relationship_version INTEGER,
    source_type VARCHAR(64),
    source_id UUID,
    idempotency_key VARCHAR(200),
    correlation_id VARCHAR(200),
    reason TEXT,
    action VARCHAR(64) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE company.carrier_relationship_outbox (
    outbox_id UUID PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT chk_cr_outbox_event CHECK (event_type IN (
        'carrier_relationship.requested',
        'carrier_relationship.accepted',
        'carrier_relationship.cancelled',
        'carrier_relationship.expired',
        'carrier_relationship.revoked'
    ))
);

CREATE OR REPLACE FUNCTION company.reject_append_only_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'append-only company relationship row is immutable'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER crr_requests_append_only
    BEFORE UPDATE OR DELETE ON company.carrier_relationship_requests
    FOR EACH ROW EXECUTE FUNCTION company.reject_append_only_mutation();

CREATE TRIGGER crr_events_append_only
    BEFORE UPDATE OR DELETE ON company.carrier_relationship_request_events
    FOR EACH ROW EXECUTE FUNCTION company.reject_append_only_mutation();

CREATE TRIGGER cr_root_append_only
    BEFORE UPDATE OR DELETE ON company.carrier_relationships
    FOR EACH ROW EXECUTE FUNCTION company.reject_append_only_mutation();

CREATE TRIGGER cr_versions_append_only
    BEFORE UPDATE OR DELETE ON company.carrier_relationship_versions
    FOR EACH ROW EXECUTE FUNCTION company.reject_append_only_mutation();

CREATE OR REPLACE FUNCTION company.enforce_request_same_tenant()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM core.companies
        WHERE id = NEW.requesting_company_id
          AND tenant_id = NEW.source_tenant_id
          AND deleted_at IS NULL
          AND status = 'ACTIVE'
    ) THEN
        RAISE EXCEPTION 'requesting company is outside the verified tenant'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM core.companies
        WHERE id = NEW.shipper_carrier_company_id
          AND tenant_id = NEW.source_tenant_id
          AND deleted_at IS NULL
          AND status = 'ACTIVE'
    ) THEN
        RAISE EXCEPTION 'carrier company is outside the verified tenant'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM core.users
        WHERE id = NEW.requesting_user_id
          AND tenant_id = NEW.source_tenant_id
          AND deleted_at IS NULL
          AND status = 'ACTIVE'
    ) THEN
        RAISE EXCEPTION 'requesting user is not an active member of the verified tenant'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER crr_requests_tenant_guard
    BEFORE INSERT ON company.carrier_relationship_requests
    FOR EACH ROW EXECUTE FUNCTION company.enforce_request_same_tenant();

CREATE OR REPLACE FUNCTION company.enforce_relationship_endpoint_tenants()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM core.companies
        WHERE id = NEW.shipper_carrier_company_id
          AND tenant_id = NEW.shipper_tenant_id
          AND deleted_at IS NULL
          AND status = 'ACTIVE'
    ) THEN
        RAISE EXCEPTION 'shipper carrier company is outside the shipper tenant'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM core.companies
        WHERE id = NEW.operating_carrier_company_id
          AND tenant_id = NEW.operating_tenant_id
          AND deleted_at IS NULL
          AND status = 'ACTIVE'
    ) THEN
        RAISE EXCEPTION 'operating company is outside the operating tenant'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER cr_root_tenant_guard
    BEFORE INSERT ON company.carrier_relationships
    FOR EACH ROW EXECUTE FUNCTION company.enforce_relationship_endpoint_tenants();

CREATE OR REPLACE FUNCTION company.reject_request_head_identity_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.relationship_request_id IS DISTINCT FROM OLD.relationship_request_id
        OR NEW.source_type IS DISTINCT FROM OLD.source_type
        OR NEW.source_id IS DISTINCT FROM OLD.source_id
        OR NEW.shipper_carrier_company_id IS DISTINCT FROM OLD.shipper_carrier_company_id
    THEN
        RAISE EXCEPTION 'request head identity is immutable'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER crr_heads_identity_guard
    BEFORE UPDATE ON company.carrier_relationship_request_heads
    FOR EACH ROW EXECUTE FUNCTION company.reject_request_head_identity_change();
