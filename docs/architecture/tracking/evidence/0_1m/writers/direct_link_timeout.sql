-- Direct link UPDATE with lock_timeout. Expect SQLSTATE 55P03 while the reader holds the link.
-- The failed transaction must not change execution_id.
BEGIN;
SET LOCAL lock_timeout = '2s';
UPDATE transport.transport_execution_active_shipments
   SET execution_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
 WHERE shipment_tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND shipment_id = '22222222-2222-2222-2222-222222222222';
COMMIT;
