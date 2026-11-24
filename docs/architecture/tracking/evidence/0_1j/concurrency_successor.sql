BEGIN;
SELECT id::text
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
UPDATE transport.transport_executions
   SET current_revision_id = '12121212-1212-1212-1212-121212121212',
       updated_at = clock_timestamp()
 WHERE id = '11111111-1111-1111-1111-111111111111';
SELECT pg_sleep(6);
COMMIT;
