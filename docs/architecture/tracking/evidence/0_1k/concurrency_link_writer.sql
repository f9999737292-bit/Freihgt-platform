BEGIN;
SELECT id::text
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
UPDATE transport.transport_execution_active_shipments
   SET execution_id = 'abababab-abab-abab-abab-abababababab'
 WHERE shipment_tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
   AND shipment_id = '22222222-2222-2222-2222-222222222222';
SELECT pg_sleep(4);
COMMIT;
