-- TMS cross-tenant PostgreSQL security foundation 0.1.
-- Preparatory schema only. Today's Project still inserts a headerless revision.
-- trg_new_activation_authorization is not created here. The same release that
-- teaches Project to write authorization evidence applies
-- infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql.
-- Roles have no passwords. freight stays the shared login until a later
-- role split. The gate refuses to install the trigger while that login can
-- bypass grants. This migration does not enable TMS_RELATIONSHIP_ACTIVATION_ENABLED.

-- Roles are cluster-global. A one-argument advisory lock is database-local, so
-- it does not stop two databases from updating the same pg_authid row. The
-- shared role row is locked instead. The comment is set only when this
-- migration creates the role, and the down script uses that marker.
DO $roles$
BEGIN
    BEGIN
        CREATE ROLE freight_migrations NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
        COMMENT ON ROLE freight_migrations IS 'tms-relationship-pins-security-foundation-v0.1';
    EXCEPTION
        WHEN unique_violation OR duplicate_object THEN
            NULL;
    END;
    BEGIN
        CREATE ROLE company_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
        COMMENT ON ROLE company_runtime IS 'tms-relationship-pins-security-foundation-v0.1';
    EXCEPTION
        WHEN unique_violation OR duplicate_object THEN
            NULL;
    END;
    BEGIN
        CREATE ROLE shipment_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
        COMMENT ON ROLE shipment_runtime IS 'tms-relationship-pins-security-foundation-v0.1';
    EXCEPTION
        WHEN unique_violation OR duplicate_object THEN
            NULL;
    END;
END
$roles$;

SELECT role_row.oid
  FROM pg_catalog.pg_authid AS role_row
 WHERE role_row.rolname IN ('company_runtime', 'freight_migrations', 'shipment_runtime')
 ORDER BY role_row.rolname
   FOR UPDATE;

ALTER ROLE freight_migrations NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
ALTER ROLE company_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
ALTER ROLE shipment_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;

GRANT USAGE ON SCHEMA company, transport, core TO freight_migrations, company_runtime, shipment_runtime;

ALTER TABLE company.carrier_relationship_outbox DROP CONSTRAINT chk_cr_outbox_event;
ALTER TABLE company.carrier_relationship_outbox ADD CONSTRAINT chk_cr_outbox_event CHECK (event_type IN (
    'carrier_relationship.requested',
    'carrier_relationship.accepted',
    'carrier_relationship.cancelled',
    'carrier_relationship.expired',
    'carrier_relationship.revoked',
    'carrier_relationship.activation_authorized'
));

ALTER TABLE transport.transport_execution_revisions
    ADD COLUMN pin_set_fingerprint text NULL;

ALTER TABLE transport.transport_execution_revisions
    ADD CONSTRAINT transport_execution_revisions_pin_fingerprint_chk CHECK (
        pin_set_fingerprint IS NULL
        OR (
            char_length(pin_set_fingerprint) = 64
            AND pin_set_fingerprint = lower(pin_set_fingerprint)
            AND translate(pin_set_fingerprint, '0123456789abcdef', '') = ''
        )
    );

CREATE TYPE company.activation_subject_input AS (
    shipment_tenant_id uuid,
    shipment_id uuid,
    source_company_id uuid
);

CREATE TABLE company.carrier_relationship_activation_authorizations (
    id uuid PRIMARY KEY,
    source_activation_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    operating_carrier_company_id uuid NOT NULL,
    contract_sha256 text NOT NULL,
    pin_set_fingerprint text NOT NULL,
    created_xact pg_catalog.xid8 NOT NULL DEFAULT pg_catalog.pg_current_xact_id(),
    created_at timestamptz NOT NULL DEFAULT pg_catalog.clock_timestamp(),
    CONSTRAINT uq_cr_activation_auth UNIQUE (operating_tenant_id, source_activation_id),
    CONSTRAINT chk_cr_activation_auth_digest CHECK (
        char_length(contract_sha256) = 64
        AND contract_sha256 = lower(contract_sha256)
        AND translate(contract_sha256, '0123456789abcdef', '') = ''
    ),
    CONSTRAINT chk_cr_activation_auth_fingerprint CHECK (
        char_length(pin_set_fingerprint) = 64
        AND pin_set_fingerprint = lower(pin_set_fingerprint)
        AND translate(pin_set_fingerprint, '0123456789abcdef', '') = ''
    )
);

CREATE TABLE company.carrier_relationship_activation_subjects (
    id uuid PRIMARY KEY,
    authorization_id uuid NOT NULL REFERENCES company.carrier_relationship_activation_authorizations (id),
    operating_tenant_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    source_company_id uuid NOT NULL,
    relationship_id uuid NOT NULL,
    locked_relationship_version_id uuid NOT NULL REFERENCES company.carrier_relationship_versions (relationship_version_id),
    relationship_version integer NOT NULL CHECK (relationship_version >= 1),
    operating_carrier_company_id uuid NOT NULL,
    CONSTRAINT uq_cr_activation_subject UNIQUE (operating_tenant_id, authorization_id, shipment_tenant_id, shipment_id),
    CONSTRAINT uq_cr_activation_subject_version UNIQUE (id, locked_relationship_version_id),
    CONSTRAINT uq_cr_activation_subject_identity UNIQUE (
        id,
        locked_relationship_version_id,
        relationship_id,
        operating_tenant_id,
        operating_carrier_company_id,
        shipment_tenant_id,
        shipment_id,
        source_company_id
    )
);

CREATE TABLE transport.transport_execution_carrier_relationship_pins (
    pin_id uuid PRIMARY KEY,
    execution_revision_id uuid NOT NULL REFERENCES transport.transport_execution_revisions (id),
    authorization_subject_id uuid NOT NULL,
    operating_tenant_id uuid NOT NULL,
    shipment_tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    source_company_id uuid NOT NULL,
    relationship_id uuid NOT NULL,
    relationship_version_id uuid NOT NULL REFERENCES company.carrier_relationship_versions (relationship_version_id),
    relationship_version integer NOT NULL CHECK (relationship_version >= 1),
    operating_carrier_company_id uuid NOT NULL,
    pin_set_fingerprint text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT pg_catalog.clock_timestamp(),
    CONSTRAINT uq_execution_relationship_pin UNIQUE (execution_revision_id, shipment_tenant_id, shipment_id),
    CONSTRAINT chk_execution_relationship_pin_fingerprint CHECK (
        char_length(pin_set_fingerprint) = 64
        AND pin_set_fingerprint = lower(pin_set_fingerprint)
        AND translate(pin_set_fingerprint, '0123456789abcdef', '') = ''
    ),
    CONSTRAINT fk_pin_subject_locked_version
        FOREIGN KEY (authorization_subject_id, relationship_version_id)
        REFERENCES company.carrier_relationship_activation_subjects (id, locked_relationship_version_id),
    CONSTRAINT fk_pin_subject_identity
        FOREIGN KEY (
            authorization_subject_id,
            relationship_version_id,
            relationship_id,
            operating_tenant_id,
            operating_carrier_company_id,
            shipment_tenant_id,
            shipment_id,
            source_company_id
        )
        REFERENCES company.carrier_relationship_activation_subjects (
            id,
            locked_relationship_version_id,
            relationship_id,
            operating_tenant_id,
            operating_carrier_company_id,
            shipment_tenant_id,
            shipment_id,
            source_company_id
        )
);

ALTER TABLE company.carrier_relationship_activation_authorizations OWNER TO freight_migrations;
ALTER TABLE company.carrier_relationship_activation_subjects OWNER TO freight_migrations;
ALTER TABLE transport.transport_execution_carrier_relationship_pins OWNER TO freight_migrations;
ALTER TYPE company.activation_subject_input OWNER TO freight_migrations;

CREATE FUNCTION company.enforce_activation_subject_head()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    head_status text;
    head_version uuid;
BEGIN
    PERFORM pg_catalog.pg_advisory_xact_lock(
        pg_catalog.hashtextextended(
            'relationship:' || NEW.shipment_tenant_id::pg_catalog.text
                || ':' || NEW.source_company_id::pg_catalog.text
                || ':' || NEW.operating_tenant_id::pg_catalog.text,
            0
        )
    );
    IF NOT EXISTS (
        SELECT 1
          FROM company.carrier_relationship_activation_authorizations AS a
         WHERE a.id OPERATOR(pg_catalog.=) NEW.authorization_id
           AND a.operating_tenant_id OPERATOR(pg_catalog.=) NEW.operating_tenant_id
           AND a.operating_carrier_company_id OPERATOR(pg_catalog.=) NEW.operating_carrier_company_id
    ) THEN
        RAISE EXCEPTION 'activation subject does not match its authorization header'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM company.carrier_relationships AS rel
         WHERE rel.relationship_id OPERATOR(pg_catalog.=) NEW.relationship_id
           AND rel.shipper_tenant_id OPERATOR(pg_catalog.=) NEW.shipment_tenant_id
           AND rel.shipper_carrier_company_id OPERATOR(pg_catalog.=) NEW.source_company_id
           AND rel.operating_tenant_id OPERATOR(pg_catalog.=) NEW.operating_tenant_id
           AND rel.operating_carrier_company_id OPERATOR(pg_catalog.=) NEW.operating_carrier_company_id
    ) THEN
        RAISE EXCEPTION 'activation subject identity does not match the relationship root'
            USING ERRCODE = '23514';
    END IF;
    SELECT h.status, h.current_version_id
      INTO head_status, head_version
      FROM company.carrier_relationship_heads AS h
     WHERE h.relationship_id OPERATOR(pg_catalog.=) NEW.relationship_id
     FOR UPDATE;
    IF head_status IS DISTINCT FROM 'ACTIVE'
       OR head_version IS DISTINCT FROM NEW.locked_relationship_version_id THEN
        RAISE EXCEPTION 'activation subject version is not the active head'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM company.carrier_relationship_versions AS v
         WHERE v.relationship_version_id OPERATOR(pg_catalog.=) NEW.locked_relationship_version_id
           AND v.relationship_id OPERATOR(pg_catalog.=) NEW.relationship_id
           AND v.relationship_version OPERATOR(pg_catalog.=) NEW.relationship_version
    ) THEN
        RAISE EXCEPTION 'activation subject version does not match the relationship version row'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$fn$;

CREATE FUNCTION company.reject_activation_evidence_mutation()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    RAISE EXCEPTION 'activation evidence is append-only'
        USING ERRCODE = '23514';
END;
$fn$;

CREATE FUNCTION transport.reject_activation_pin_mutation()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    RAISE EXCEPTION 'activation pin is immutable'
        USING ERRCODE = '23514';
END;
$fn$;

CREATE FUNCTION transport.enforce_activation_pin_binding()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM transport.transport_execution_revisions AS r
         WHERE r.id OPERATOR(pg_catalog.=) NEW.execution_revision_id
    ) THEN
        RETURN NEW;
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM transport.transport_execution_revisions AS r
         WHERE r.id OPERATOR(pg_catalog.=) NEW.execution_revision_id
           AND r.operating_tenant_id OPERATOR(pg_catalog.=) NEW.operating_tenant_id
           AND r.pin_set_fingerprint OPERATOR(pg_catalog.=) NEW.pin_set_fingerprint
    ) THEN
        RAISE EXCEPTION 'pin does not match the execution revision'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM company.carrier_relationship_versions AS v
         WHERE v.relationship_version_id OPERATOR(pg_catalog.=) NEW.relationship_version_id
           AND v.relationship_id OPERATOR(pg_catalog.=) NEW.relationship_id
           AND v.relationship_version OPERATOR(pg_catalog.=) NEW.relationship_version
    ) THEN
        RAISE EXCEPTION 'pin relationship version does not match the relationship root'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM company.carrier_relationships AS rel
         WHERE rel.relationship_id OPERATOR(pg_catalog.=) NEW.relationship_id
           AND rel.shipper_tenant_id OPERATOR(pg_catalog.=) NEW.shipment_tenant_id
           AND rel.shipper_carrier_company_id OPERATOR(pg_catalog.=) NEW.source_company_id
           AND rel.operating_tenant_id OPERATOR(pg_catalog.=) NEW.operating_tenant_id
           AND rel.operating_carrier_company_id OPERATOR(pg_catalog.=) NEW.operating_carrier_company_id
    ) THEN
        RAISE EXCEPTION 'pin company identity does not match the relationship root'
            USING ERRCODE = '23514';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM transport.transport_execution_carrier_relationship_pins AS pin
         WHERE pin.execution_revision_id OPERATOR(pg_catalog.=) NEW.execution_revision_id
           AND pin.pin_set_fingerprint IS DISTINCT FROM NEW.pin_set_fingerprint
    ) THEN
        RAISE EXCEPTION 'activation revision has a second pin fingerprint'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$fn$;

-- xmin is xid. pg_current_xact_id() is xid8. The xid cast keeps the low 32 bits,
-- which is the value stored on a row inserted by this top-level transaction.
-- Text comparison is not used: xid8 text includes the epoch, so it diverges
-- from xmin after the first xid wrap. A committed row cannot match the current
-- xid. PostgreSQL freezes or removes every tuple before that xid is reused,
-- and FrozenTransactionId is never assigned as a current transaction id.
CREATE FUNCTION transport.revision_inserted_in_current_xact(p_xmin pg_catalog.xid)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY INVOKER
SET search_path = pg_catalog
AS $fn$
    SELECT p_xmin OPERATOR(pg_catalog.=) pg_catalog.pg_current_xact_id()::pg_catalog.xid;
$fn$;

-- The rewrite trigger is an invoker so current_user() is the caller. Pin rows
-- stay unreadable by shipment_runtime, so the comparison runs as the owner.
CREATE FUNCTION transport.revision_pin_fingerprint_conflicts(p_revision_id uuid, p_fingerprint text)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
    SELECT EXISTS (
        SELECT 1
          FROM transport.transport_execution_carrier_relationship_pins AS pin
         WHERE pin.execution_revision_id OPERATOR(pg_catalog.=) p_revision_id
           AND pin.pin_set_fingerprint IS DISTINCT FROM p_fingerprint
    );
$fn$;

CREATE FUNCTION transport.reject_revision_fingerprint_rewrite()
RETURNS trigger
LANGUAGE plpgsql
SECURITY INVOKER
SET search_path = pg_catalog
AS $fn$
BEGIN
    -- execution_id and contract_sha256 are fixed at INSERT. A later UPDATE in
    -- the same transaction is still a different execution or a different
    -- contract, including the interval after pins are attached. status and
    -- updated_at stay writable so CreateSuccessorRevision can supersede the row.
    IF transport.revision_inserted_in_current_xact(OLD.xmin) THEN
        IF NEW.execution_id IS DISTINCT FROM OLD.execution_id
           OR NEW.contract_sha256 IS DISTINCT FROM OLD.contract_sha256 THEN
            RAISE EXCEPTION 'activation revision identity is immutable'
                USING ERRCODE = '23514';
        END IF;
        IF NEW.pin_set_fingerprint IS DISTINCT FROM OLD.pin_set_fingerprint
           AND pg_catalog.current_user() IS DISTINCT FROM 'freight_migrations' THEN
            RAISE EXCEPTION 'revision pin_set_fingerprint is immutable'
                USING ERRCODE = '23514';
        END IF;
        IF transport.revision_pin_fingerprint_conflicts(NEW.id, NEW.pin_set_fingerprint) THEN
            RAISE EXCEPTION 'revision pin_set_fingerprint is immutable'
                USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.execution_id IS DISTINCT FROM OLD.execution_id
       OR NEW.operating_tenant_id IS DISTINCT FROM OLD.operating_tenant_id
       OR NEW.source_route_plan_id IS DISTINCT FROM OLD.source_route_plan_id
       OR NEW.source_route_plan_version IS DISTINCT FROM OLD.source_route_plan_version
       OR NEW.source_activation_id IS DISTINCT FROM OLD.source_activation_id
       OR NEW.source_activation_version IS DISTINCT FROM OLD.source_activation_version
       OR NEW.evaluation_fingerprint IS DISTINCT FROM OLD.evaluation_fingerprint
       OR NEW.planning_mode IS DISTINCT FROM OLD.planning_mode
       OR NEW.supersedes_revision_id IS DISTINCT FROM OLD.supersedes_revision_id
       OR NEW.version IS DISTINCT FROM OLD.version
       OR NEW.contract_sha256 IS DISTINCT FROM OLD.contract_sha256
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.pin_set_fingerprint IS DISTINCT FROM OLD.pin_set_fingerprint THEN
        RAISE EXCEPTION 'committed revision identity is immutable'
            USING ERRCODE = '23514';
    END IF;
    IF transport.revision_pin_fingerprint_conflicts(NEW.id, NEW.pin_set_fingerprint) THEN
        RAISE EXCEPTION 'revision pin_set_fingerprint is immutable'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$fn$;

CREATE FUNCTION transport.enforce_new_activation_authorization()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    revision_id uuid;
    current_source uuid;
    current_supersedes uuid;
    current_operating uuid;
    current_execution uuid;
    current_digest text;
    current_xmin pg_catalog.xid;
    header_id uuid;
    header_digest text;
    header_carrier uuid;
    subject_count integer;
    pin_count integer;
    surviving_headerless integer;
BEGIN
    IF TG_OP OPERATOR(pg_catalog.=) 'DELETE' THEN
        revision_id := OLD.id;
    ELSE
        revision_id := NEW.id;
    END IF;
    SELECT r.source_activation_id, r.supersedes_revision_id, r.operating_tenant_id,
           r.execution_id, r.contract_sha256, r.xmin
      INTO current_source, current_supersedes, current_operating,
           current_execution, current_digest, current_xmin
      FROM transport.transport_execution_revisions AS r
     WHERE r.id OPERATOR(pg_catalog.=) revision_id;
    IF NOT FOUND THEN
        IF TG_OP OPERATOR(pg_catalog.=) 'DELETE' THEN
            RETURN NULL;
        END IF;
        SELECT pg_catalog.count(*)
          INTO surviving_headerless
          FROM transport.transport_execution_revisions AS r
         WHERE transport.revision_inserted_in_current_xact(r.xmin)
           AND r.created_at OPERATOR(pg_catalog.>=) pg_catalog.transaction_timestamp()
           AND r.source_activation_id IS NOT NULL
           AND r.supersedes_revision_id IS NULL
           AND NOT EXISTS (
                SELECT 1
                  FROM company.carrier_relationship_activation_authorizations AS a
                 WHERE a.operating_tenant_id OPERATOR(pg_catalog.=) r.operating_tenant_id
                   AND a.source_activation_id OPERATOR(pg_catalog.=) r.source_activation_id
                   AND a.created_xact OPERATOR(pg_catalog.=) pg_catalog.pg_current_xact_id()
                   AND transport.revision_inserted_in_current_xact(a.xmin)
           );
        IF surviving_headerless OPERATOR(pg_catalog.>) 0 THEN
            RAISE EXCEPTION 'new activation revision has no authorization header'
                USING ERRCODE = '23514';
        END IF;
        RETURN NULL;
    END IF;
    -- Successors and headerless legacy revisions stay outside this branch.
    -- The deferred INSERT trigger reads the row at COMMIT, so a carrier or
    -- pin change made after attach is visible here.
    IF current_source IS NOT NULL AND current_supersedes IS NULL THEN
        SELECT a.id, a.contract_sha256, a.operating_carrier_company_id
          INTO header_id, header_digest, header_carrier
          FROM company.carrier_relationship_activation_authorizations AS a
         WHERE a.operating_tenant_id OPERATOR(pg_catalog.=) current_operating
           AND a.source_activation_id OPERATOR(pg_catalog.=) current_source
           AND a.created_xact OPERATOR(pg_catalog.=) pg_catalog.pg_current_xact_id()
           AND transport.revision_inserted_in_current_xact(a.xmin);
        IF NOT FOUND THEN
            RAISE EXCEPTION 'new activation revision has no authorization header'
                USING ERRCODE = '23514';
        END IF;
        IF NOT transport.revision_inserted_in_current_xact(current_xmin) THEN
            RAISE EXCEPTION 'new activation revision has no authorization header'
                USING ERRCODE = '23514';
        END IF;
        IF EXISTS (
            SELECT 1
              FROM company.carrier_relationship_activation_subjects AS s
              JOIN company.carrier_relationship_heads AS h
                ON h.relationship_id OPERATOR(pg_catalog.=) s.relationship_id
             WHERE s.authorization_id OPERATOR(pg_catalog.=) header_id
               AND (
                    h.status IS DISTINCT FROM 'ACTIVE'
                    OR h.current_version_id IS DISTINCT FROM s.locked_relationship_version_id
               )
        ) THEN
            RAISE EXCEPTION 'RELATIONSHIP_REVOKED'
                USING ERRCODE = '23514';
        END IF;
        IF current_digest IS DISTINCT FROM header_digest THEN
            RAISE EXCEPTION 'activation revision contract does not match the authorization header'
                USING ERRCODE = '23514';
        END IF;
        IF NOT EXISTS (
            SELECT 1
              FROM transport.transport_executions AS e
             WHERE e.id OPERATOR(pg_catalog.=) current_execution
               AND e.operating_tenant_id OPERATOR(pg_catalog.=) current_operating
               AND e.carrier_company_id OPERATOR(pg_catalog.=) header_carrier
        ) THEN
            RAISE EXCEPTION 'activation revision execution does not match the authorization carrier'
                USING ERRCODE = '23514';
        END IF;
        SELECT pg_catalog.count(*)
          INTO subject_count
          FROM company.carrier_relationship_activation_subjects AS s
         WHERE s.authorization_id OPERATOR(pg_catalog.=) header_id;
        SELECT pg_catalog.count(*)
          INTO pin_count
          FROM transport.transport_execution_carrier_relationship_pins AS p
         WHERE p.execution_revision_id OPERATOR(pg_catalog.=) revision_id;
        IF subject_count OPERATOR(pg_catalog.<) 1
           OR pin_count IS DISTINCT FROM subject_count
           OR EXISTS (
                SELECT 1
                  FROM company.carrier_relationship_activation_subjects AS s
                 WHERE s.authorization_id OPERATOR(pg_catalog.=) header_id
                   AND NOT EXISTS (
                        SELECT 1
                          FROM transport.transport_execution_carrier_relationship_pins AS p
                         WHERE p.execution_revision_id OPERATOR(pg_catalog.=) revision_id
                           AND p.authorization_subject_id OPERATOR(pg_catalog.=) s.id
                           AND p.relationship_version_id OPERATOR(pg_catalog.=) s.locked_relationship_version_id
                           AND p.operating_tenant_id OPERATOR(pg_catalog.=) s.operating_tenant_id
                           AND p.operating_carrier_company_id OPERATOR(pg_catalog.=) s.operating_carrier_company_id
                   )
           )
           OR EXISTS (
                SELECT 1
                  FROM transport.transport_execution_carrier_relationship_pins AS p
                 WHERE p.execution_revision_id OPERATOR(pg_catalog.=) revision_id
                   AND NOT EXISTS (
                        SELECT 1
                          FROM company.carrier_relationship_activation_subjects AS s
                         WHERE s.authorization_id OPERATOR(pg_catalog.=) header_id
                           AND s.id OPERATOR(pg_catalog.=) p.authorization_subject_id
                           AND s.locked_relationship_version_id OPERATOR(pg_catalog.=) p.relationship_version_id
                   )
           ) THEN
            RAISE EXCEPTION 'activation pin set does not match the authorization header'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NULL;
END;
$fn$;

CREATE FUNCTION company.authorize_carrier_relationship_activation(
    p_source_activation_id uuid,
    p_operating_tenant_id uuid,
    p_operating_carrier_company_id uuid,
    p_contract_sha256 text,
    p_pin_set_fingerprint text,
    p_subjects company.activation_subject_input[]
) RETURNS TABLE (
    outcome text,
    authorization_id uuid,
    execution_id uuid,
    revision_id uuid
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_digest text;
    v_n integer := 0;
    v_i integer;
    v_j integer;
    v_root_count integer;
    v_root_company uuid;
    v_root_id uuid;
    v_status text;
    v_version_id uuid;
    v_version integer;
    v_tenants uuid[] := ARRAY[]::uuid[];
    v_shipments uuid[] := ARRAY[]::uuid[];
    v_carriers uuid[] := ARRAY[]::uuid[];
    v_relationships uuid[] := ARRAY[]::uuid[];
    v_version_ids uuid[] := ARRAY[]::uuid[];
    v_versions integer[] := ARRAY[]::integer[];
    subject record;
BEGIN
    IF p_source_activation_id IS NULL
       OR p_operating_tenant_id IS NULL
       OR p_operating_carrier_company_id IS NULL
       OR p_subjects IS NULL
       OR pg_catalog.cardinality(p_subjects) IS NULL
       OR pg_catalog.cardinality(p_subjects) OPERATOR(pg_catalog.<) 1 THEN
        RAISE EXCEPTION 'activation subject set is empty'
            USING ERRCODE = '23514';
    END IF;
    IF p_contract_sha256 IS NULL
       OR char_length(p_contract_sha256) OPERATOR(pg_catalog.<>) 64
       OR p_contract_sha256 IS DISTINCT FROM pg_catalog.lower(p_contract_sha256)
       OR pg_catalog.translate(p_contract_sha256, '0123456789abcdef', '') OPERATOR(pg_catalog.<>) ''
       OR p_pin_set_fingerprint IS NULL
       OR char_length(p_pin_set_fingerprint) OPERATOR(pg_catalog.<>) 64
       OR p_pin_set_fingerprint IS DISTINCT FROM pg_catalog.lower(p_pin_set_fingerprint)
       OR pg_catalog.translate(p_pin_set_fingerprint, '0123456789abcdef', '') OPERATOR(pg_catalog.<>) '' THEN
        RAISE EXCEPTION 'activation digest is not lowercase sha-256'
            USING ERRCODE = '23514';
    END IF;

    SELECT r.contract_sha256, r.id, r.execution_id
      INTO v_digest, revision_id, execution_id
      FROM transport.transport_execution_revisions AS r
     WHERE r.operating_tenant_id OPERATOR(pg_catalog.=) p_operating_tenant_id
       AND r.source_activation_id OPERATOR(pg_catalog.=) p_source_activation_id;
    IF FOUND THEN
        IF v_digest IS DISTINCT FROM p_contract_sha256 THEN
            RAISE EXCEPTION 'ACTIVATION_BODY_CONFLICT'
                USING ERRCODE = '23514';
        END IF;
        outcome := 'REPLAY';
        SELECT a.id
          INTO authorization_id
          FROM company.carrier_relationship_activation_authorizations AS a
         WHERE a.operating_tenant_id OPERATOR(pg_catalog.=) p_operating_tenant_id
           AND a.source_activation_id OPERATOR(pg_catalog.=) p_source_activation_id;
        RETURN NEXT;
        RETURN;
    END IF;

    FOR subject IN
        SELECT (s).shipment_tenant_id AS shipment_tenant_id,
               (s).shipment_id AS shipment_id,
               (s).source_company_id AS source_company_id
          FROM pg_catalog.unnest(p_subjects) AS s
         ORDER BY pg_catalog.lower((s).shipment_tenant_id::pg_catalog.text),
                  pg_catalog.lower((s).source_company_id::pg_catalog.text),
                  (s).shipment_id
    LOOP
        v_n := v_n OPERATOR(pg_catalog.+) 1;
        IF subject.shipment_tenant_id IS NULL
           OR subject.shipment_id IS NULL
           OR subject.source_company_id IS NULL THEN
            RAISE EXCEPTION 'activation subject set is empty'
                USING ERRCODE = '23514';
        END IF;
        FOR v_j IN 1 .. v_n - 1 LOOP
            IF v_tenants[v_j] OPERATOR(pg_catalog.=) subject.shipment_tenant_id
               AND v_shipments[v_j] OPERATOR(pg_catalog.=) subject.shipment_id THEN
                RAISE EXCEPTION 'activation subject set is duplicated'
                    USING ERRCODE = '23514';
            END IF;
        END LOOP;
        v_tenants[v_n] := subject.shipment_tenant_id;
        v_shipments[v_n] := subject.shipment_id;
        v_carriers[v_n] := subject.source_company_id;
        PERFORM pg_catalog.pg_advisory_xact_lock(
            pg_catalog.hashtextextended(
                'relationship:' || subject.shipment_tenant_id::pg_catalog.text
                    || ':' || subject.source_company_id::pg_catalog.text
                    || ':' || p_operating_tenant_id::pg_catalog.text,
                0
            )
        );
    END LOOP;

    SELECT r.contract_sha256, r.id, r.execution_id
      INTO v_digest, revision_id, execution_id
      FROM transport.transport_execution_revisions AS r
     WHERE r.operating_tenant_id OPERATOR(pg_catalog.=) p_operating_tenant_id
       AND r.source_activation_id OPERATOR(pg_catalog.=) p_source_activation_id;
    IF FOUND THEN
        IF v_digest IS DISTINCT FROM p_contract_sha256 THEN
            RAISE EXCEPTION 'ACTIVATION_BODY_CONFLICT'
                USING ERRCODE = '23514';
        END IF;
        outcome := 'REPLAY';
        SELECT a.id
          INTO authorization_id
          FROM company.carrier_relationship_activation_authorizations AS a
         WHERE a.operating_tenant_id OPERATOR(pg_catalog.=) p_operating_tenant_id
           AND a.source_activation_id OPERATOR(pg_catalog.=) p_source_activation_id;
        RETURN NEXT;
        RETURN;
    END IF;

    FOR v_i IN 1..v_n LOOP
        SELECT pg_catalog.count(*), pg_catalog.min(rel.operating_carrier_company_id::pg_catalog.text)::uuid
          INTO v_root_count, v_root_company
          FROM company.carrier_relationships AS rel
         WHERE rel.shipper_tenant_id OPERATOR(pg_catalog.=) v_tenants[v_i]
           AND rel.shipper_carrier_company_id OPERATOR(pg_catalog.=) v_carriers[v_i]
           AND rel.operating_tenant_id OPERATOR(pg_catalog.=) p_operating_tenant_id;
        IF v_root_count OPERATOR(pg_catalog.=) 0 THEN
            RAISE EXCEPTION 'RELATIONSHIP_NOT_FOUND'
                USING ERRCODE = '23514';
        END IF;
        IF v_root_count OPERATOR(pg_catalog.>) 1 THEN
            RAISE EXCEPTION 'RELATIONSHIP_AMBIGUOUS'
                USING ERRCODE = '23514';
        END IF;
        IF v_root_company IS DISTINCT FROM p_operating_carrier_company_id THEN
            RAISE EXCEPTION 'RELATIONSHIP_OPERATING_COMPANY_MISMATCH'
                USING ERRCODE = '23514';
        END IF;
        SELECT rel.relationship_id
          INTO v_root_id
          FROM company.carrier_relationships AS rel
         WHERE rel.shipper_tenant_id OPERATOR(pg_catalog.=) v_tenants[v_i]
           AND rel.shipper_carrier_company_id OPERATOR(pg_catalog.=) v_carriers[v_i]
           AND rel.operating_tenant_id OPERATOR(pg_catalog.=) p_operating_tenant_id;
        v_relationships[v_i] := v_root_id;
    END LOOP;

    FOR v_i IN 1..v_n LOOP
        SELECT h.status, h.current_version_id, h.current_version
          INTO v_status, v_version_id, v_version
          FROM company.carrier_relationship_heads AS h
         WHERE h.relationship_id OPERATOR(pg_catalog.=) v_relationships[v_i]
         FOR UPDATE;
        IF NOT FOUND OR v_status IS DISTINCT FROM 'ACTIVE' THEN
            RAISE EXCEPTION 'RELATIONSHIP_REVOKED'
                USING ERRCODE = '23514';
        END IF;
        v_version_ids[v_i] := v_version_id;
        v_versions[v_i] := v_version;
    END LOOP;

    authorization_id := pg_catalog.gen_random_uuid();
    INSERT INTO company.carrier_relationship_activation_authorizations (
        id, source_activation_id, operating_tenant_id, operating_carrier_company_id,
        contract_sha256, pin_set_fingerprint, created_xact
    ) VALUES (
        authorization_id, p_source_activation_id, p_operating_tenant_id, p_operating_carrier_company_id,
        p_contract_sha256, p_pin_set_fingerprint, pg_catalog.pg_current_xact_id()
    );
    FOR v_i IN 1..v_n LOOP
        INSERT INTO company.carrier_relationship_activation_subjects (
            id, authorization_id, operating_tenant_id, shipment_tenant_id, shipment_id,
            source_company_id, relationship_id, locked_relationship_version_id,
            relationship_version, operating_carrier_company_id
        ) VALUES (
            pg_catalog.gen_random_uuid(), authorization_id, p_operating_tenant_id,
            v_tenants[v_i], v_shipments[v_i], v_carriers[v_i], v_relationships[v_i],
            v_version_ids[v_i], v_versions[v_i], p_operating_carrier_company_id
        );
    END LOOP;
    outcome := 'AUTHORIZED';
    execution_id := NULL;
    revision_id := NULL;
    RETURN NEXT;
    RETURN;
END;
$fn$;

CREATE FUNCTION transport.attach_activation_pins(p_revision_id uuid)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_auth uuid;
    v_finger text;
    v_digest text;
    v_header_digest text;
    v_company uuid;
    v_tenant uuid;
    v_execution uuid;
    v_execution_company uuid;
    v_supersedes uuid;
    v_source uuid;
    v_count integer := 0;
    subject record;
BEGIN
    SELECT r.contract_sha256, r.operating_tenant_id, r.execution_id, r.supersedes_revision_id,
           r.source_activation_id, e.carrier_company_id
      INTO v_digest, v_tenant, v_execution, v_supersedes, v_source, v_execution_company
      FROM transport.transport_execution_revisions AS r
      JOIN transport.transport_executions AS e
        ON e.id OPERATOR(pg_catalog.=) r.execution_id
       AND e.operating_tenant_id OPERATOR(pg_catalog.=) r.operating_tenant_id
     WHERE r.id OPERATOR(pg_catalog.=) p_revision_id
       AND transport.revision_inserted_in_current_xact(r.xmin);
    IF NOT FOUND OR v_supersedes IS NOT NULL THEN
        RAISE EXCEPTION 'activation pins require a revision created in this transaction'
            USING ERRCODE = '23514';
    END IF;
    SELECT a.id, a.pin_set_fingerprint, a.contract_sha256, a.operating_carrier_company_id
      INTO v_auth, v_finger, v_header_digest, v_company
      FROM company.carrier_relationship_activation_authorizations AS a
     WHERE a.operating_tenant_id OPERATOR(pg_catalog.=) v_tenant
       AND a.source_activation_id OPERATOR(pg_catalog.=) v_source
       AND a.created_xact OPERATOR(pg_catalog.=) pg_catalog.pg_current_xact_id()
       AND transport.revision_inserted_in_current_xact(a.xmin);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'activation pin requires authorization from this transaction'
            USING ERRCODE = '23514';
    END IF;
    IF v_header_digest IS DISTINCT FROM v_digest THEN
        RAISE EXCEPTION 'ACTIVATION_BODY_CONFLICT'
            USING ERRCODE = '23514';
    END IF;
    IF v_execution_company IS DISTINCT FROM v_company THEN
        RAISE EXCEPTION 'RELATIONSHIP_OPERATING_COMPANY_MISMATCH'
            USING ERRCODE = '23514';
    END IF;
    PERFORM h.relationship_id
      FROM company.carrier_relationship_heads AS h
      JOIN company.carrier_relationship_activation_subjects AS s
        ON s.relationship_id OPERATOR(pg_catalog.=) h.relationship_id
     WHERE s.authorization_id OPERATOR(pg_catalog.=) v_auth
     FOR UPDATE OF h;
    IF EXISTS (
        SELECT 1
          FROM company.carrier_relationship_activation_subjects AS s
          JOIN company.carrier_relationship_heads AS h
            ON h.relationship_id OPERATOR(pg_catalog.=) s.relationship_id
         WHERE s.authorization_id OPERATOR(pg_catalog.=) v_auth
           AND (
                h.status IS DISTINCT FROM 'ACTIVE'
                OR h.current_version_id IS DISTINCT FROM s.locked_relationship_version_id
           )
    ) THEN
        RAISE EXCEPTION 'RELATIONSHIP_REVOKED'
            USING ERRCODE = '23514';
    END IF;
    UPDATE transport.transport_execution_revisions
       SET pin_set_fingerprint = v_finger
     WHERE id OPERATOR(pg_catalog.=) p_revision_id
       AND (pin_set_fingerprint IS NULL OR pin_set_fingerprint OPERATOR(pg_catalog.=) v_finger);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'revision pin_set_fingerprint is immutable'
            USING ERRCODE = '23514';
    END IF;
    FOR subject IN
        SELECT s.id, s.operating_tenant_id, s.shipment_tenant_id, s.shipment_id, s.source_company_id,
               s.relationship_id, s.locked_relationship_version_id, s.relationship_version,
               s.operating_carrier_company_id
          FROM company.carrier_relationship_activation_subjects AS s
         WHERE s.authorization_id OPERATOR(pg_catalog.=) v_auth
         ORDER BY s.shipment_tenant_id, s.shipment_id
    LOOP
        INSERT INTO transport.transport_execution_carrier_relationship_pins (
            pin_id, execution_revision_id, authorization_subject_id, operating_tenant_id,
            shipment_tenant_id, shipment_id, source_company_id, relationship_id,
            relationship_version_id, relationship_version, operating_carrier_company_id,
            pin_set_fingerprint
        ) VALUES (
            pg_catalog.gen_random_uuid(), p_revision_id, subject.id, subject.operating_tenant_id,
            subject.shipment_tenant_id, subject.shipment_id, subject.source_company_id,
            subject.relationship_id, subject.locked_relationship_version_id, subject.relationship_version,
            subject.operating_carrier_company_id, v_finger
        );
        v_count := v_count OPERATOR(pg_catalog.+) 1;
    END LOOP;
    IF v_count OPERATOR(pg_catalog.<) 1 THEN
        RAISE EXCEPTION 'activation pin requires authorization header'
            USING ERRCODE = '23514';
    END IF;
    RETURN v_count;
END;
$fn$;

CREATE FUNCTION transport.propagate_execution_relationship_pins(p_successor_revision_id uuid)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_source uuid;
    v_tenant uuid;
    v_execution uuid;
    v_finger text;
    v_company uuid;
    v_count integer := 0;
    v_source_tenant uuid;
    v_source_execution uuid;
    v_source_count integer;
    pin record;
BEGIN
    SELECT r.supersedes_revision_id, r.operating_tenant_id, r.execution_id, e.carrier_company_id
      INTO v_source, v_tenant, v_execution, v_company
      FROM transport.transport_execution_revisions AS r
      JOIN transport.transport_executions AS e
        ON e.id OPERATOR(pg_catalog.=) r.execution_id
       AND e.operating_tenant_id OPERATOR(pg_catalog.=) r.operating_tenant_id
     WHERE r.id OPERATOR(pg_catalog.=) p_successor_revision_id
       AND transport.revision_inserted_in_current_xact(r.xmin);
    IF NOT FOUND OR v_source IS NULL THEN
        RAISE EXCEPTION 'successor pin copy requires a revision created in this transaction'
            USING ERRCODE = '23514';
    END IF;
    SELECT source.execution_id, source.operating_tenant_id, source.pin_set_fingerprint
      INTO v_source_execution, v_source_tenant, v_finger
      FROM transport.transport_execution_revisions AS source
     WHERE source.id OPERATOR(pg_catalog.=) v_source;
    IF NOT FOUND
       OR v_source_execution IS DISTINCT FROM v_execution
       OR v_source_tenant IS DISTINCT FROM v_tenant THEN
        RAISE EXCEPTION 'successor pin copy source revision is not the previous revision of this execution'
            USING ERRCODE = '23514';
    END IF;
    IF NOT EXISTS (
        SELECT 1
          FROM transport.transport_executions AS execution
         WHERE execution.id OPERATOR(pg_catalog.=) v_execution
           AND execution.operating_tenant_id OPERATOR(pg_catalog.=) v_tenant
           AND execution.carrier_company_id OPERATOR(pg_catalog.=) v_company
    ) THEN
        RAISE EXCEPTION 'successor pin copy does not match the execution'
            USING ERRCODE = '23514';
    END IF;
    FOR pin IN
        SELECT p.authorization_subject_id, p.operating_tenant_id, p.shipment_tenant_id, p.shipment_id,
               p.source_company_id, p.relationship_id, p.relationship_version_id, p.relationship_version,
               p.operating_carrier_company_id, p.pin_set_fingerprint
          FROM transport.transport_execution_carrier_relationship_pins AS p
         WHERE p.execution_revision_id OPERATOR(pg_catalog.=) v_source
         ORDER BY p.shipment_tenant_id, p.shipment_id
    LOOP
        IF pin.operating_tenant_id IS DISTINCT FROM v_tenant
           OR pin.operating_carrier_company_id IS DISTINCT FROM v_company
           OR pin.pin_set_fingerprint IS DISTINCT FROM v_finger THEN
            RAISE EXCEPTION 'successor pin copy does not match the execution'
                USING ERRCODE = '23514';
        END IF;
        IF v_count OPERATOR(pg_catalog.=) 0 THEN
            UPDATE transport.transport_execution_revisions
               SET pin_set_fingerprint = v_finger
             WHERE id OPERATOR(pg_catalog.=) p_successor_revision_id
               AND (pin_set_fingerprint IS NULL OR pin_set_fingerprint OPERATOR(pg_catalog.=) v_finger);
            IF NOT FOUND THEN
                RAISE EXCEPTION 'revision pin_set_fingerprint is immutable'
                    USING ERRCODE = '23514';
            END IF;
        END IF;
        INSERT INTO transport.transport_execution_carrier_relationship_pins (
            pin_id, execution_revision_id, authorization_subject_id, operating_tenant_id,
            shipment_tenant_id, shipment_id, source_company_id, relationship_id,
            relationship_version_id, relationship_version, operating_carrier_company_id,
            pin_set_fingerprint
        ) VALUES (
            pg_catalog.gen_random_uuid(), p_successor_revision_id, pin.authorization_subject_id,
            pin.operating_tenant_id, pin.shipment_tenant_id, pin.shipment_id, pin.source_company_id,
            pin.relationship_id, pin.relationship_version_id, pin.relationship_version,
            pin.operating_carrier_company_id, pin.pin_set_fingerprint
        );
        v_count := v_count OPERATOR(pg_catalog.+) 1;
    END LOOP;
    SELECT pg_catalog.count(*)
      INTO v_source_count
      FROM transport.transport_execution_carrier_relationship_pins AS source_pin
     WHERE source_pin.execution_revision_id OPERATOR(pg_catalog.=) v_source;
    IF v_count IS DISTINCT FROM v_source_count THEN
        RAISE EXCEPTION 'successor pin copy does not match the historical pin set'
            USING ERRCODE = '23514';
    END IF;
    RETURN v_count;
END;
$fn$;

CREATE FUNCTION company.record_activation_audit(
    p_authorization_id uuid,
    p_actor_user_id uuid,
    p_actor_tenant_id uuid,
    p_selected_company_id uuid,
    p_correlation_id text
) RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
DECLARE
    v_source uuid;
    v_tenant uuid;
    v_company uuid;
    v_digest text;
    v_finger text;
    v_revision uuid;
BEGIN
    SELECT a.source_activation_id, a.operating_tenant_id, a.operating_carrier_company_id,
           a.contract_sha256, a.pin_set_fingerprint
      INTO v_source, v_tenant, v_company, v_digest, v_finger
      FROM company.carrier_relationship_activation_authorizations AS a
     WHERE a.id OPERATOR(pg_catalog.=) p_authorization_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'activation audit requires authorization header'
            USING ERRCODE = '23514';
    END IF;
    SELECT r.id
      INTO v_revision
      FROM transport.transport_execution_revisions AS r
     WHERE r.operating_tenant_id OPERATOR(pg_catalog.=) v_tenant
       AND r.source_activation_id OPERATOR(pg_catalog.=) v_source
       AND r.supersedes_revision_id IS NULL;
    IF NOT FOUND OR NOT EXISTS (
        SELECT 1
          FROM transport.transport_execution_carrier_relationship_pins AS pin
         WHERE pin.execution_revision_id OPERATOR(pg_catalog.=) v_revision
    ) THEN
        RAISE EXCEPTION 'activation audit requires pins'
            USING ERRCODE = '23514';
    END IF;
    INSERT INTO company.carrier_relationship_audit (
        audit_id, actor_user_id, actor_tenant_id, selected_company_id,
        relationship_request_id, relationship_id, relationship_version,
        source_type, source_id, idempotency_key, correlation_id, reason, action
    ) VALUES (
        pg_catalog.gen_random_uuid(), p_actor_user_id, p_actor_tenant_id, p_selected_company_id,
        NULL, NULL, NULL, NULL, NULL, NULL, p_correlation_id, NULL, 'ACTIVATION_AUTHORIZED'
    );
    INSERT INTO company.carrier_relationship_outbox (outbox_id, event_type, aggregate_id, payload)
    VALUES (
        pg_catalog.gen_random_uuid(),
        'carrier_relationship.activation_authorized',
        p_authorization_id,
        pg_catalog.jsonb_build_object(
            'authorization_id', p_authorization_id,
            'source_activation_id', v_source,
            'operating_tenant_id', v_tenant,
            'operating_carrier_company_id', v_company,
            'contract_sha256', v_digest,
            'pin_set_fingerprint', v_finger,
            'locked_relationship_version_ids', (
                SELECT pg_catalog.jsonb_agg(s.locked_relationship_version_id ORDER BY s.shipment_tenant_id, s.shipment_id)
                  FROM company.carrier_relationship_activation_subjects AS s
                 WHERE s.authorization_id OPERATOR(pg_catalog.=) p_authorization_id
            )
        )
    );
END;
$fn$;

CREATE FUNCTION transport.assert_activation_enforcement_role_split()
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_roles AS r
         WHERE r.rolname OPERATOR(pg_catalog.=) 'freight'
           AND (r.rolsuper OR r.rolbypassrls)
    ) THEN
        RAISE EXCEPTION 'shared freight login can bypass activation enforcement'
            USING ERRCODE = '42501';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_class AS c
          JOIN pg_catalog.pg_roles AS owner_role ON owner_role.oid OPERATOR(pg_catalog.=) c.relowner
         WHERE owner_role.rolname OPERATOR(pg_catalog.=) 'freight'
           AND c.oid IN (
                'transport.transport_execution_revisions'::pg_catalog.regclass,
                'transport.transport_execution_carrier_relationship_pins'::pg_catalog.regclass,
                'company.carrier_relationship_activation_authorizations'::pg_catalog.regclass,
                'company.carrier_relationship_activation_subjects'::pg_catalog.regclass
           )
    ) THEN
        RAISE EXCEPTION 'shared freight login can bypass activation enforcement'
            USING ERRCODE = '42501';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM pg_catalog.pg_class AS c
          CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) AS acl
          JOIN pg_catalog.pg_roles AS grantee_role ON grantee_role.oid OPERATOR(pg_catalog.=) acl.grantee
         WHERE grantee_role.rolname OPERATOR(pg_catalog.=) 'freight'
           AND acl.privilege_type IN ('INSERT', 'UPDATE', 'DELETE')
           AND c.oid IN (
                'transport.transport_execution_carrier_relationship_pins'::pg_catalog.regclass,
                'company.carrier_relationship_activation_authorizations'::pg_catalog.regclass,
                'company.carrier_relationship_activation_subjects'::pg_catalog.regclass
           )
    ) THEN
        RAISE EXCEPTION 'shared freight login can bypass activation enforcement'
            USING ERRCODE = '42501';
    END IF;
END;
$fn$;

ALTER FUNCTION company.enforce_activation_subject_head() OWNER TO freight_migrations;
ALTER FUNCTION company.reject_activation_evidence_mutation() OWNER TO freight_migrations;
ALTER FUNCTION transport.reject_activation_pin_mutation() OWNER TO freight_migrations;
ALTER FUNCTION transport.enforce_activation_pin_binding() OWNER TO freight_migrations;
ALTER FUNCTION transport.revision_pin_fingerprint_conflicts(uuid, text) OWNER TO freight_migrations;
ALTER FUNCTION transport.reject_revision_fingerprint_rewrite() OWNER TO freight_migrations;
ALTER FUNCTION transport.revision_inserted_in_current_xact(pg_catalog.xid) OWNER TO freight_migrations;
ALTER FUNCTION transport.enforce_new_activation_authorization() OWNER TO freight_migrations;
ALTER FUNCTION company.authorize_carrier_relationship_activation(
    uuid, uuid, uuid, text, text, company.activation_subject_input[]
) OWNER TO freight_migrations;
ALTER FUNCTION transport.attach_activation_pins(uuid) OWNER TO freight_migrations;
ALTER FUNCTION transport.propagate_execution_relationship_pins(uuid) OWNER TO freight_migrations;
ALTER FUNCTION company.record_activation_audit(uuid, uuid, uuid, uuid, text) OWNER TO freight_migrations;
ALTER FUNCTION transport.assert_activation_enforcement_role_split() OWNER TO freight_migrations;

CREATE TRIGGER trg_cr_activation_subject_head
    BEFORE INSERT ON company.carrier_relationship_activation_subjects
    FOR EACH ROW
    EXECUTE FUNCTION company.enforce_activation_subject_head();

CREATE TRIGGER trg_cr_activation_auth_immutable
    BEFORE UPDATE OR DELETE ON company.carrier_relationship_activation_authorizations
    FOR EACH ROW
    EXECUTE FUNCTION company.reject_activation_evidence_mutation();

CREATE TRIGGER trg_cr_activation_subject_immutable
    BEFORE UPDATE OR DELETE ON company.carrier_relationship_activation_subjects
    FOR EACH ROW
    EXECUTE FUNCTION company.reject_activation_evidence_mutation();

CREATE TRIGGER trg_execution_relationship_pin_binding
    BEFORE INSERT ON transport.transport_execution_carrier_relationship_pins
    FOR EACH ROW
    EXECUTE FUNCTION transport.enforce_activation_pin_binding();

CREATE TRIGGER trg_execution_relationship_pin_immutable
    BEFORE UPDATE OR DELETE ON transport.transport_execution_carrier_relationship_pins
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_activation_pin_mutation();

CREATE TRIGGER trg_revision_pin_fingerprint_immutable
    BEFORE UPDATE ON transport.transport_execution_revisions
    FOR EACH ROW
    EXECUTE FUNCTION transport.reject_revision_fingerprint_rewrite();

REVOKE ALL ON FUNCTION company.enforce_activation_subject_head() FROM PUBLIC;
REVOKE ALL ON FUNCTION company.reject_activation_evidence_mutation() FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.reject_activation_pin_mutation() FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.enforce_activation_pin_binding() FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.revision_pin_fingerprint_conflicts(uuid, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.reject_revision_fingerprint_rewrite() FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.revision_inserted_in_current_xact(pg_catalog.xid) FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.enforce_new_activation_authorization() FROM PUBLIC;
REVOKE ALL ON FUNCTION company.authorize_carrier_relationship_activation(
    uuid, uuid, uuid, text, text, company.activation_subject_input[]
) FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.attach_activation_pins(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.propagate_execution_relationship_pins(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION company.record_activation_audit(uuid, uuid, uuid, uuid, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.assert_activation_enforcement_role_split() FROM PUBLIC;
REVOKE ALL ON TYPE company.activation_subject_input FROM PUBLIC;
REVOKE ALL ON TABLE company.carrier_relationship_activation_authorizations FROM PUBLIC;
REVOKE ALL ON TABLE company.carrier_relationship_activation_subjects FROM PUBLIC;
REVOKE ALL ON TABLE transport.transport_execution_carrier_relationship_pins FROM PUBLIC;

GRANT EXECUTE ON FUNCTION company.authorize_carrier_relationship_activation(
    uuid, uuid, uuid, text, text, company.activation_subject_input[]
) TO shipment_runtime;
GRANT EXECUTE ON FUNCTION transport.attach_activation_pins(uuid) TO shipment_runtime;
GRANT EXECUTE ON FUNCTION transport.propagate_execution_relationship_pins(uuid) TO shipment_runtime;
GRANT EXECUTE ON FUNCTION company.record_activation_audit(uuid, uuid, uuid, uuid, text) TO shipment_runtime;
GRANT EXECUTE ON FUNCTION transport.enforce_new_activation_authorization() TO shipment_runtime;
GRANT EXECUTE ON FUNCTION transport.revision_pin_fingerprint_conflicts(uuid, text) TO shipment_runtime;
GRANT EXECUTE ON FUNCTION transport.reject_revision_fingerprint_rewrite() TO shipment_runtime;
GRANT EXECUTE ON FUNCTION transport.revision_inserted_in_current_xact(pg_catalog.xid) TO shipment_runtime;
GRANT USAGE ON TYPE company.activation_subject_input TO shipment_runtime;
GRANT EXECUTE ON FUNCTION transport.assert_activation_enforcement_role_split() TO freight_migrations;

GRANT SELECT, UPDATE ON company.carrier_relationship_heads TO freight_migrations;
GRANT SELECT ON company.carrier_relationships TO freight_migrations;
GRANT SELECT ON company.carrier_relationship_versions TO freight_migrations;
GRANT INSERT ON company.carrier_relationship_audit TO freight_migrations;
GRANT INSERT ON company.carrier_relationship_outbox TO freight_migrations;
GRANT SELECT, UPDATE ON transport.transport_execution_revisions TO freight_migrations;
GRANT SELECT ON transport.transport_executions TO freight_migrations;

GRANT SELECT, INSERT, UPDATE ON transport.transport_executions TO shipment_runtime;
GRANT SELECT, INSERT ON transport.transport_execution_revisions TO shipment_runtime;
GRANT UPDATE (
    id,
    execution_id,
    operating_tenant_id,
    source_route_plan_id,
    source_route_plan_version,
    source_activation_id,
    source_activation_version,
    evaluation_fingerprint,
    planning_mode,
    supersedes_revision_id,
    status,
    version,
    contract_sha256,
    created_at,
    updated_at
) ON transport.transport_execution_revisions TO shipment_runtime;
REVOKE DELETE ON transport.transport_execution_revisions FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON transport.transport_execution_carrier_relationship_pins FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_authorizations FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_subjects FROM shipment_runtime;

GRANT SELECT, INSERT, UPDATE ON company.carrier_relationships TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_versions TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_heads TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_requests TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_request_events TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_request_heads TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_request_idempotency TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_acceptance_idempotency TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_revoke_idempotency TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_cancel_idempotency TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_audit TO company_runtime;
GRANT SELECT, INSERT, UPDATE ON company.carrier_relationship_outbox TO company_runtime;
GRANT SELECT ON core.companies TO company_runtime;
GRANT SELECT ON core.tenants TO company_runtime;
GRANT SELECT ON core.users TO company_runtime;
GRANT SELECT ON core.company_memberships TO company_runtime;
GRANT SELECT ON core.user_roles TO company_runtime;
GRANT SELECT ON core.roles TO company_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_authorizations FROM company_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_subjects FROM company_runtime;
REVOKE INSERT, UPDATE, DELETE ON transport.transport_execution_carrier_relationship_pins FROM company_runtime;
