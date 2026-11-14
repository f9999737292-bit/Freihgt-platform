-- Privilege set for model B. UPDATE for the lock owner is only updated_at.
-- owner_epoch is readable and is not an UPDATE column for that role.

GRANT USAGE ON SCHEMA transport TO assignment_lock_owner;
GRANT USAGE ON SCHEMA transport TO binding_authority_fence;
GRANT USAGE ON SCHEMA transport TO tracking_runtime;
GRANT USAGE ON SCHEMA transport TO column_select_only;
GRANT USAGE ON SCHEMA transport TO command_probe;
GRANT USAGE ON SCHEMA transport TO writer_probe;
GRANT USAGE ON SCHEMA transport TO inherit_probe;
GRANT USAGE ON SCHEMA transport TO set_probe;
GRANT USAGE ON SCHEMA tracking TO binding_authority_fence;

GRANT SELECT (id) ON transport.shipments TO column_select_only;

GRANT SELECT (id, tenant_id, driver_id, vehicle_id, version, deleted_at)
    ON transport.shipments TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.shipments TO assignment_lock_owner;

GRANT SELECT (id, tenant_id, status, deleted_at) ON transport.drivers TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.drivers TO assignment_lock_owner;

GRANT SELECT (id, tenant_id, status) ON transport.vehicles TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.vehicles TO assignment_lock_owner;

GRANT SELECT (id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, owner_epoch)
    ON transport.transport_executions TO assignment_lock_owner;
GRANT UPDATE (updated_at) ON transport.transport_executions TO assignment_lock_owner;

GRANT SELECT (shipment_tenant_id, shipment_id, execution_id)
    ON transport.transport_execution_active_shipments TO assignment_lock_owner;
GRANT SELECT (id, execution_id, operating_tenant_id, status, version)
    ON transport.transport_execution_revisions TO assignment_lock_owner;

GRANT SELECT (
    decision_id, shipment_tenant_id, operating_tenant_id, shipment_id, driver_id, vehicle_id,
    execution_id, current_revision_id, shipment_version, revision_version, owner_epoch,
    use_status, consumed_at, binding_id
) ON transport.binding_authority_decisions TO binding_authority_fence;
GRANT UPDATE (use_status, consumed_at, binding_id)
    ON transport.binding_authority_decisions TO binding_authority_fence;

GRANT SELECT (id, driver_id, vehicle_id, version),
      UPDATE (driver_id, vehicle_id, version, updated_at)
    ON transport.shipments TO writer_probe;

GRANT SELECT, UPDATE, INSERT ON transport.transport_executions TO command_probe;
GRANT SELECT, UPDATE, INSERT ON transport.transport_execution_revisions TO command_probe;
GRANT SELECT, UPDATE, INSERT ON transport.transport_execution_active_shipments TO command_probe;
GRANT SELECT, UPDATE ON transport.shipments TO command_probe;

GRANT CREATE ON SCHEMA transport TO assignment_lock_owner;
\ir function.sql
REVOKE CREATE ON SCHEMA transport FROM assignment_lock_owner;

REVOKE ALL ON FUNCTION transport.read_binding_assignment(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION transport.read_binding_assignment_0_1j(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transport.read_binding_assignment(uuid, uuid) TO binding_authority_fence;
GRANT EXECUTE ON FUNCTION transport.read_binding_assignment_0_1j(uuid, uuid) TO binding_authority_fence;
