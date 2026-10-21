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
DROP FUNCTION IF EXISTS transport.enforce_new_activation_authorization();
DROP FUNCTION IF EXISTS transport.reject_revision_fingerprint_rewrite();
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

DO $down$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'freight_migrations') THEN
        DROP OWNED BY freight_migrations;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'company_runtime') THEN
        DROP OWNED BY company_runtime;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'shipment_runtime') THEN
        DROP OWNED BY shipment_runtime;
    END IF;
END
$down$;

DROP ROLE IF EXISTS shipment_runtime;
DROP ROLE IF EXISTS company_runtime;
DROP ROLE IF EXISTS freight_migrations;
