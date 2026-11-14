BEGIN;
SELECT execution_id::text
    || '|' || coalesce(owner_epoch::text, 'null')
    || '|' || coalesce(current_revision_id::text, 'null')
    || '|' || coalesce(revision_version::text, 'null')
    || '|' || coalesce(operating_tenant_id::text, 'null')
  FROM transport.read_binding_assignment(
      '11111111-1111-1111-1111-111111111111',
      '22222222-2222-2222-2222-222222222222');
SELECT pg_sleep(6);
COMMIT;
