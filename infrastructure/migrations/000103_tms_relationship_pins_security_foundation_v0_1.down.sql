-- Reverse the preparatory security foundation. Historical executions stay.
-- Drops the enforcement trigger too, so a database that applied the gate can roll back.

DROP TRIGGER IF EXISTS trg_new_activation_authorization ON transport.transport_execution_revisions;
DROP TRIGGER IF EXISTS trg_revision_pin_fingerprint_immutable ON transport.transport_execution_revisions;

DROP TABLE IF EXISTS transport.transport_execution_carrier_relationship_pins;
DROP TABLE IF EXISTS company.carrier_relationship_activation_subjects;
DROP TABLE IF EXISTS company.carrier_relationship_activation_authorizations;

DROP FUNCTION IF EXISTS company.authorize_carrier_relationship_activation(uuid, uuid, uuid, text, text, company.activation_subject_input[]);
DROP FUNCTION IF EXISTS transport.attach_activation_pins(uuid);
DROP FUNCTION IF EXISTS transport.propagate_execution_relationship_pins(uuid);
DROP FUNCTION IF EXISTS company.record_activation_audit(uuid, uuid, uuid, uuid, text);
DROP FUNCTION IF EXISTS transport.assert_activation_enforcement_role_split();
DROP FUNCTION IF EXISTS transport.revision_inserted_in_current_xact(pg_catalog.xid);
DROP FUNCTION IF EXISTS transport.enforce_new_activation_authorization();
DROP FUNCTION IF EXISTS transport.reject_revision_fingerprint_rewrite();
DROP FUNCTION IF EXISTS transport.revision_pin_fingerprint_conflicts(uuid, text);
DROP FUNCTION IF EXISTS transport.enforce_activation_pin_binding();
DROP FUNCTION IF EXISTS transport.reject_activation_pin_mutation();
DROP FUNCTION IF EXISTS company.enforce_activation_subject_head();
DROP FUNCTION IF EXISTS company.reject_activation_evidence_mutation();

DROP TYPE IF EXISTS company.activation_subject_input;

ALTER TABLE transport.transport_execution_revisions
    DROP CONSTRAINT IF EXISTS transport_execution_revisions_pin_fingerprint_chk;
ALTER TABLE transport.transport_execution_revisions
    DROP COLUMN IF EXISTS pin_set_fingerprint;

DELETE FROM company.carrier_relationship_outbox
 WHERE event_type = 'carrier_relationship.activation_authorized';
DELETE FROM company.carrier_relationship_audit
 WHERE action = 'ACTIVATION_AUTHORIZED';

ALTER TABLE company.carrier_relationship_outbox DROP CONSTRAINT IF EXISTS chk_cr_outbox_event;
ALTER TABLE company.carrier_relationship_outbox ADD CONSTRAINT chk_cr_outbox_event CHECK (event_type IN (
    'carrier_relationship.requested',
    'carrier_relationship.accepted',
    'carrier_relationship.cancelled',
    'carrier_relationship.expired',
    'carrier_relationship.revoked'
));

-- Revoke only grants created by this migration. Skip a table that an earlier
-- down already removed, and skip a role that another database still needs.
DO $revoke$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'freight_migrations') THEN
        REVOKE ALL ON SCHEMA company, transport, core FROM freight_migrations;
        IF to_regclass('company.carrier_relationship_heads') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_heads FROM freight_migrations;
        END IF;
        IF to_regclass('company.carrier_relationships') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationships FROM freight_migrations;
        END IF;
        IF to_regclass('company.carrier_relationship_versions') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_versions FROM freight_migrations;
        END IF;
        IF to_regclass('company.carrier_relationship_audit') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_audit FROM freight_migrations;
        END IF;
        IF to_regclass('company.carrier_relationship_outbox') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_outbox FROM freight_migrations;
        END IF;
        IF to_regclass('transport.transport_execution_revisions') IS NOT NULL THEN
            REVOKE ALL ON TABLE transport.transport_execution_revisions FROM freight_migrations;
        END IF;
        IF to_regclass('transport.transport_executions') IS NOT NULL THEN
            REVOKE ALL ON TABLE transport.transport_executions FROM freight_migrations;
        END IF;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'shipment_runtime') THEN
        REVOKE ALL ON SCHEMA company, transport, core FROM shipment_runtime;
        IF to_regclass('transport.transport_execution_revisions') IS NOT NULL THEN
            REVOKE ALL ON TABLE transport.transport_execution_revisions FROM shipment_runtime;
        END IF;
        IF to_regclass('transport.transport_executions') IS NOT NULL THEN
            REVOKE ALL ON TABLE transport.transport_executions FROM shipment_runtime;
        END IF;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'company_runtime') THEN
        REVOKE ALL ON SCHEMA company, transport, core FROM company_runtime;
        IF to_regclass('company.carrier_relationship_heads') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_heads FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationships') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationships FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_versions') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_versions FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_requests') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_requests FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_request_events') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_request_events FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_request_heads') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_request_heads FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_request_idempotency') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_request_idempotency FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_acceptance_idempotency') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_acceptance_idempotency FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_revoke_idempotency') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_revoke_idempotency FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_cancel_idempotency') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_cancel_idempotency FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_audit') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_audit FROM company_runtime;
        END IF;
        IF to_regclass('company.carrier_relationship_outbox') IS NOT NULL THEN
            REVOKE ALL ON TABLE company.carrier_relationship_outbox FROM company_runtime;
        END IF;
        IF to_regclass('core.companies') IS NOT NULL THEN
            REVOKE ALL ON TABLE core.companies FROM company_runtime;
        END IF;
        IF to_regclass('core.tenants') IS NOT NULL THEN
            REVOKE ALL ON TABLE core.tenants FROM company_runtime;
        END IF;
        IF to_regclass('core.users') IS NOT NULL THEN
            REVOKE ALL ON TABLE core.users FROM company_runtime;
        END IF;
        IF to_regclass('core.company_memberships') IS NOT NULL THEN
            REVOKE ALL ON TABLE core.company_memberships FROM company_runtime;
        END IF;
        IF to_regclass('core.user_roles') IS NOT NULL THEN
            REVOKE ALL ON TABLE core.user_roles FROM company_runtime;
        END IF;
        IF to_regclass('core.roles') IS NOT NULL THEN
            REVOKE ALL ON TABLE core.roles FROM company_runtime;
        END IF;
    END IF;
END
$revoke$;

-- Roles are cluster-global. Drop one only when this migration created it and
-- pg_shdepend shows no remaining owner or grant dependency in any database.
-- A role that already existed, or a role still used by another database, stays.
DO $down$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'shipment_runtime')
       AND EXISTS (
            SELECT 1
              FROM pg_catalog.pg_shdescription AS description
              JOIN pg_catalog.pg_authid AS role_row
                ON role_row.oid = description.objoid
             WHERE description.classoid = 'pg_catalog.pg_authid'::pg_catalog.regclass
               AND role_row.rolname = 'shipment_runtime'
               AND description.description = 'tms-relationship-pins-security-foundation-v0.1'
       )
       AND NOT EXISTS (
            SELECT 1
              FROM pg_catalog.pg_shdepend AS dependency
              JOIN pg_catalog.pg_authid AS role_row
                ON role_row.oid = dependency.refobjid
             WHERE dependency.refclassid = 'pg_catalog.pg_authid'::pg_catalog.regclass
               AND role_row.rolname = 'shipment_runtime'
       ) THEN
        DROP ROLE shipment_runtime;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'company_runtime')
       AND EXISTS (
            SELECT 1
              FROM pg_catalog.pg_shdescription AS description
              JOIN pg_catalog.pg_authid AS role_row
                ON role_row.oid = description.objoid
             WHERE description.classoid = 'pg_catalog.pg_authid'::pg_catalog.regclass
               AND role_row.rolname = 'company_runtime'
               AND description.description = 'tms-relationship-pins-security-foundation-v0.1'
       )
       AND NOT EXISTS (
            SELECT 1
              FROM pg_catalog.pg_shdepend AS dependency
              JOIN pg_catalog.pg_authid AS role_row
                ON role_row.oid = dependency.refobjid
             WHERE dependency.refclassid = 'pg_catalog.pg_authid'::pg_catalog.regclass
               AND role_row.rolname = 'company_runtime'
       ) THEN
        DROP ROLE company_runtime;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'freight_migrations')
       AND EXISTS (
            SELECT 1
              FROM pg_catalog.pg_shdescription AS description
              JOIN pg_catalog.pg_authid AS role_row
                ON role_row.oid = description.objoid
             WHERE description.classoid = 'pg_catalog.pg_authid'::pg_catalog.regclass
               AND role_row.rolname = 'freight_migrations'
               AND description.description = 'tms-relationship-pins-security-foundation-v0.1'
       )
       AND NOT EXISTS (
            SELECT 1
              FROM pg_catalog.pg_shdepend AS dependency
              JOIN pg_catalog.pg_authid AS role_row
                ON role_row.oid = dependency.refobjid
             WHERE dependency.refclassid = 'pg_catalog.pg_authid'::pg_catalog.regclass
               AND role_row.rolname = 'freight_migrations'
       ) THEN
        DROP ROLE freight_migrations;
    END IF;
END
$down$;
