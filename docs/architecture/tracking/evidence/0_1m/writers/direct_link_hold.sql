-- Direct link UPDATE. Sleeps before COMMIT so the reader can observe the wait.
BEGIN;
UPDATE transport.transport_execution_active_shipments
   SET execution_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
 WHERE shipment_tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND shipment_id = '22222222-2222-2222-2222-222222222222';
SELECT pg_sleep(15);
COMMIT;
