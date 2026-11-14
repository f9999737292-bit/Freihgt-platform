BEGIN;
SELECT id FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
SELECT pg_sleep(18);
UPDATE transport.transport_executions
   SET id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
 WHERE id = '11111111-1111-1111-1111-111111111111';
COMMIT;
