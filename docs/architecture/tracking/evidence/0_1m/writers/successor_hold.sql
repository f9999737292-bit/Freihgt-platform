-- Successor revision. Sleeps after the execution FOR UPDATE and before the revision write.
BEGIN;
SELECT operating_tenant_id, current_revision_id
  FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111'
   FOR UPDATE;
SELECT pg_sleep(15);
UPDATE transport.transport_execution_revisions
   SET status = 'SUPERSEDED', updated_at = now()
 WHERE id = '55555555-5555-5555-5555-555555555555'
   AND status = 'ACTIVE';
INSERT INTO transport.transport_execution_revisions (
    id, execution_id, operating_tenant_id, status, version
) VALUES (
    '55555555-5555-5555-5555-555555555556', '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', 1
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555556', updated_at = now()
 WHERE id = '11111111-1111-1111-1111-111111111111';
COMMIT;
