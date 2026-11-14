BEGIN;
SET lock_timeout = '2s';
UPDATE transport.transport_execution_active_shipments
   SET execution_id = 'abababab-abab-abab-abab-abababababab'
 WHERE shipment_tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
   AND shipment_id = '22222222-2222-2222-2222-222222222222';
COMMIT;
