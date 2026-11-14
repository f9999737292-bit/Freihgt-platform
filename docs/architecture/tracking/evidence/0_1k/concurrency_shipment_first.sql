-- Opposite of the test function: shipment before execution.
BEGIN;
SELECT id::text
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   FOR UPDATE;
SELECT pg_sleep(3);
SELECT id::text
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
COMMIT;
