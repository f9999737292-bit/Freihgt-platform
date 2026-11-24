-- Restore the main shipment, execution, revision, and link after a concurrency script.

-- Restore the main fixture. session_replication_role skips the epoch and revision
-- triggers so the admin can put the bigint epoch back. A command writer cannot.

SET session_replication_role = replica;
UPDATE transport.transport_execution_revisions
   SET operating_tenant_id = 'cccccccc-cccc-cccc-cccc-cccccccccccc',
       status = 'SUPERSEDED'
 WHERE id = '12121212-1212-1212-1212-121212121212';
UPDATE transport.transport_execution_revisions
   SET operating_tenant_id = 'cccccccc-cccc-cccc-cccc-cccccccccccc',
       status = 'ACTIVE'
 WHERE id = '55555555-5555-5555-5555-555555555555';
UPDATE transport.transport_executions
   SET operating_tenant_id = 'cccccccc-cccc-cccc-cccc-cccccccccccc',
       current_revision_id = '55555555-5555-5555-5555-555555555555',
       owner_epoch = 4,
       updated_at = '2000-01-01+00'
 WHERE id = '11111111-1111-1111-1111-111111111111';
UPDATE transport.transport_execution_active_shipments
   SET execution_id = '11111111-1111-1111-1111-111111111111'
 WHERE shipment_tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
   AND shipment_id = '22222222-2222-2222-2222-222222222222';
UPDATE transport.shipments
   SET driver_id = '33333333-3333-3333-3333-333333333333',
       vehicle_id = '44444444-4444-4444-4444-444444444444',
       version = 3,
       updated_at = '2000-01-01+00'
 WHERE id = '22222222-2222-2222-2222-222222222222';
UPDATE transport.binding_authority_decisions
   SET use_status = 'ISSUED',
       consumed_at = NULL,
       binding_id = NULL
 WHERE decision_id IN (
    '66666666-6666-6666-6666-666666666666',
    '66666666-6666-6666-6666-666666666675'
 );
SET session_replication_role = origin;
