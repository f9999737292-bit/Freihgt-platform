-- Opposite of the test function: active link before execution.
BEGIN;
SELECT execution_id::text
  FROM transport.transport_execution_active_shipments
 WHERE shipment_tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
   AND shipment_id = '22222222-2222-2222-2222-222222222222'
   FOR UPDATE;
SELECT pg_sleep(3);
SELECT id::text
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
COMMIT;
