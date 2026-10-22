-- TMS activation runtime grants 0.1.
-- Lets shipment_runtime run Project and CreateSuccessorRevision.
-- Does not create trg_new_activation_authorization.
-- Does not enable TMS_RELATIONSHIP_ACTIVATION_ENABLED.
-- Does not create passwords or LOGIN roles.
-- The shared freight login remains a deployment blocker. The enforcement
-- gate in infrastructure/postgres/tms_activation_enforcement_gate_v0.1.sql
-- stays unapplied until a later joint release.

GRANT SELECT, UPDATE ON transport.shipments TO shipment_runtime;
GRANT SELECT, UPDATE ON transport.cargoes TO shipment_runtime;
GRANT SELECT, UPDATE ON transport.drivers TO shipment_runtime;
GRANT SELECT, UPDATE ON transport.vehicles TO shipment_runtime;
GRANT SELECT ON transport.locations TO shipment_runtime;
GRANT SELECT ON core.companies TO shipment_runtime;

GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_participants TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_active_shipments TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_stops TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_revision_stops TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_actions TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_revision_actions TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_commands TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.transport_execution_command_audit TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.driver_stop_tasks TO shipment_runtime;
GRANT SELECT, INSERT, UPDATE ON transport.shipment_event_outbox TO shipment_runtime;

REVOKE INSERT, UPDATE, DELETE ON transport.transport_execution_carrier_relationship_pins FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_authorizations FROM shipment_runtime;
REVOKE INSERT, UPDATE, DELETE ON company.carrier_relationship_activation_subjects FROM shipment_runtime;
REVOKE DELETE ON transport.transport_execution_revisions FROM shipment_runtime;
