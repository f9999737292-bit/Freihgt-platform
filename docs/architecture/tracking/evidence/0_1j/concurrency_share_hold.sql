BEGIN;
SELECT shipment_id::text
  FROM transport.read_binding_assignment(
      '11111111-1111-1111-1111-111111111111',
      '22222222-2222-2222-2222-222222222222');
SELECT pg_sleep(8);
COMMIT;
