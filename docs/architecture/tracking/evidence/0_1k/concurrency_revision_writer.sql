BEGIN;
SELECT id::text
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
UPDATE transport.transport_execution_revisions
   SET status = 'SUPERSEDED'
 WHERE id = '55555555-5555-5555-5555-555555555555';
UPDATE transport.transport_execution_revisions
   SET status = 'ACTIVE'
 WHERE id = '12121212-1212-1212-1212-121212121212';
UPDATE transport.transport_executions
   SET current_revision_id = '12121212-1212-1212-1212-121212121212',
       updated_at = clock_timestamp()
 WHERE id = '11111111-1111-1111-1111-111111111111';
SELECT pg_sleep(4);
COMMIT;
