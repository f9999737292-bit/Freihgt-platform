BEGIN;
SELECT coalesce(execution_id::text, 'null')
    || '|' || coalesce(owner_epoch::text, 'null')
  FROM transport.read_binding_assignment(
      NULL,
      '22222222-2222-2222-2222-222222222222');
SELECT pg_sleep(4);
ROLLBACK;
