BEGIN;
SELECT coalesce(execution_id::text, 'null')
  FROM transport.read_binding_assignment(
      NULL,
      '00000000-0000-0000-0000-000000000007');
SELECT pg_sleep(6);
SELECT coalesce(execution_id::text, 'null')
  FROM transport.read_binding_assignment(
      NULL,
      '00000000-0000-0000-0000-000000000007');
SELECT count(*)::text
  FROM pg_locks AS l
  JOIN pg_class AS c ON c.oid = l.relation
 WHERE l.pid = pg_backend_pid()
   AND c.relname = 'transport_executions'
   AND l.mode = 'RowShareLock';
ROLLBACK;
