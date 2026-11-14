-- Same flag-off Project shape as project_off_occupied.sql.
-- The shipment has no active slot, so the link INSERT commits a new execution.
BEGIN;
SELECT version
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222223'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL;
SELECT version
  FROM transport.cargoes
 WHERE id = '44444444-4444-4444-4444-444444444445'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL;
INSERT INTO transport.transport_executions (
    id, operating_tenant_id, driver_id, vehicle_id, current_revision_id, event_seq
) VALUES (
    '12121212-1212-1212-1212-121212121212', '00000000-0000-0000-0000-00000000000b',
    '77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333', NULL, 0
);
INSERT INTO transport.transport_execution_revisions (
    id, execution_id, operating_tenant_id, status, version
) VALUES (
    '55555555-5555-5555-5555-555555555558', '12121212-1212-1212-1212-121212121212',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', 1
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555558', updated_at = now()
 WHERE id = '12121212-1212-1212-1212-121212121212'
   AND operating_tenant_id = '00000000-0000-0000-0000-00000000000b';
INSERT INTO transport.transport_execution_active_shipments (
    shipment_tenant_id, shipment_id, execution_id
) VALUES (
    '00000000-0000-0000-0000-00000000000a', '22222222-2222-2222-2222-222222222223',
    '12121212-1212-1212-1212-121212121212'
);
COMMIT;
