-- Driver-stop command. Sleeps after the plain link SELECT so the lock set can be read.
-- The active-link table must not carry a row lock from this session.
BEGIN;
SELECT operating_tenant_id
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
SELECT id
  FROM transport.transport_execution_stops
 WHERE id = '66666666-6666-6666-6666-666666666666'
   FOR UPDATE;
SELECT id
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL
   FOR UPDATE;
UPDATE transport.shipments
   SET status = 'LOADED', version = version + 1, updated_at = now()
 WHERE id = '22222222-2222-2222-2222-222222222222'
   AND tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND deleted_at IS NULL
   AND version = 1;
UPDATE transport.transport_executions
   SET event_seq = event_seq + 1, updated_at = now()
 WHERE id = '11111111-1111-1111-1111-111111111111';
SELECT execution_id
  FROM transport.transport_execution_active_shipments
 WHERE shipment_tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND shipment_id = '22222222-2222-2222-2222-222222222222';
SELECT pg_sleep(15);
COMMIT;
