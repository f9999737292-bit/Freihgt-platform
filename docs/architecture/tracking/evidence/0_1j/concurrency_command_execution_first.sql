-- deadlock_timeout is not set here. This pair does not form a lock cycle.
-- PostgreSQL 16 rejected SET deadlock_timeout for command_probe with SQLSTATE 42501.
BEGIN;
SELECT id::text
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
SELECT pg_sleep(6);
SELECT id::text
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   FOR UPDATE;
COMMIT;
