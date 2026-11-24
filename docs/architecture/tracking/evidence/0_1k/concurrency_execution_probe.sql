BEGIN;
SET lock_timeout = '2s';
SELECT id::text
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
ROLLBACK;
