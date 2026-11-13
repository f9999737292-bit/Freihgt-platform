BEGIN;
SELECT coalesce(execution_id::text, 'null')
  FROM transport.read_binding_assignment(
      NULL,
      '00000000-0000-0000-0000-000000000008');
SELECT count(*)::text
  FROM pg_locks AS l
  JOIN pg_class AS c ON c.oid = l.relation
 WHERE l.pid = pg_backend_pid()
   AND c.relname = 'transport_executions'
   AND l.mode = 'RowShareLock';
DO $do$
DECLARE
    v_execution uuid;
BEGIN
    SELECT execution_id
      INTO v_execution
      FROM transport.read_binding_assignment(
          NULL,
          '00000000-0000-0000-0000-000000000008');
    IF v_execution IS NOT NULL THEN
        RAISE EXCEPTION 'REAUTHORIZATION_REQUIRED' USING ERRCODE = '40001';
    END IF;
END
$do$;
ROLLBACK;
