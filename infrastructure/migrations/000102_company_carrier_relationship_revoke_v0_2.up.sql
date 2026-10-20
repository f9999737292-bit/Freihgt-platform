-- Revoke idempotency, a nullable reason on the immutable successor, and the relationship head transition guard.
-- Existing ACTIVE versions stay valid because the new column is null. Accept may still move REVOKED back to ACTIVE.

CREATE TABLE company.carrier_relationship_revoke_idempotency (
    relationship_id UUID NOT NULL REFERENCES company.carrier_relationships(relationship_id),
    actor_tenant_id UUID NOT NULL,
    selected_company_id UUID NOT NULL,
    idempotency_key VARCHAR(200) NOT NULL,
    body_hash BYTEA NOT NULL,
    relationship_version INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (relationship_id, actor_tenant_id, selected_company_id, idempotency_key),
    CONSTRAINT chk_cr_revoke_idempotency_key CHECK (length(btrim(idempotency_key)) > 0),
    CONSTRAINT chk_cr_revoke_body_hash_len CHECK (octet_length(body_hash) = 32)
);

CREATE OR REPLACE FUNCTION company.enforce_revoke_idempotency_tenant()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    shipper_tenant UUID;
    operating_tenant UUID;
BEGIN
    SELECT shipper_tenant_id, operating_tenant_id
      INTO shipper_tenant, operating_tenant
    FROM company.carrier_relationships
    WHERE relationship_id = NEW.relationship_id;
    IF shipper_tenant IS NULL
       OR (NEW.actor_tenant_id IS DISTINCT FROM shipper_tenant AND NEW.actor_tenant_id IS DISTINCT FROM operating_tenant) THEN
        RAISE EXCEPTION 'revoke actor tenant is not a relationship endpoint'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM core.companies
        WHERE id = NEW.selected_company_id
          AND tenant_id = NEW.actor_tenant_id
          AND deleted_at IS NULL
          AND status = 'ACTIVE'
    ) THEN
        RAISE EXCEPTION 'selected company is outside the acting tenant'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER cr_revoke_idempotency_tenant_guard
    BEFORE INSERT ON company.carrier_relationship_revoke_idempotency
    FOR EACH ROW EXECUTE FUNCTION company.enforce_revoke_idempotency_tenant();

ALTER TABLE company.carrier_relationship_versions
    ADD COLUMN revoke_reason TEXT NULL;

ALTER TABLE company.carrier_relationship_versions
    ADD CONSTRAINT chk_cr_version_revoke_reason CHECK (
        (status = 'ACTIVE' AND revoke_reason IS NULL)
        OR (
            status = 'REVOKED'
            AND (revoke_reason IS NULL OR char_length(revoke_reason) <= 500)
        )
    );

CREATE OR REPLACE FUNCTION company.reject_relationship_head_illegal_transition()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = NEW.status THEN
        IF NEW.current_version IS DISTINCT FROM OLD.current_version
           OR NEW.current_version_id IS DISTINCT FROM OLD.current_version_id THEN
            RAISE EXCEPTION 'relationship head same-status rewrite is illegal'
                USING ERRCODE = '55000';
        END IF;
        RETURN NEW;
    END IF;
    IF (
            (OLD.status = 'ACTIVE' AND NEW.status = 'REVOKED')
            OR (OLD.status = 'REVOKED' AND NEW.status = 'ACTIVE')
       )
       AND NEW.current_version = OLD.current_version + 1
       AND NEW.current_version_id IS DISTINCT FROM OLD.current_version_id THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'relationship head transition is illegal'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER cr_heads_transition_guard
    BEFORE UPDATE ON company.carrier_relationship_heads
    FOR EACH ROW EXECUTE FUNCTION company.reject_relationship_head_illegal_transition();
