-- Direct UPDATE of execution_id. No product statement does this.
-- The privilege is GRANT SELECT, INSERT, UPDATE on
-- transport.transport_execution_active_shipments at
-- 000104_tms_activation_runtime_grants_v0_1.up.sql:18.
-- This session does not lock transport.transport_executions.
-- It does not close J-DML-001.
BEGIN;
UPDATE transport.transport_execution_active_shipments
   SET execution_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
 WHERE shipment_tenant_id = '00000000-0000-0000-0000-00000000000a'
   AND shipment_id = '22222222-2222-2222-2222-222222222222';
COMMIT;
