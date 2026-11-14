-- TransportExecutionCommandRepository.Execute transport_execution_command_repository.go:50-156.
-- Transaction BEGIN :62, COMMIT :153.
-- Lock order: execution FOR UPDATE :70, stop FOR UPDATE (stand-in for FOR UPDATE OF stop :1121),
-- shipment FOR UPDATE :1147, shipment UPDATE :1177, execution event_seq UPDATE :1225,
-- plain SELECT of the active link :660.
-- The link SELECT has no FOR UPDATE. Command insert, audit, and outbox are omitted.
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
   SET status = 'LOADED',
       version = version + 1,
       updated_at = now()
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
COMMIT;
