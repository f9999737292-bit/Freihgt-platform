BEGIN;
UPDATE transport.transport_execution_revisions
   SET status = 'SUPERSEDED' WHERE id = '55555555-5555-5555-5555-555555555555' AND status = 'ACTIVE';
INSERT INTO transport.transport_execution_revisions (id, execution_id, operating_tenant_id, status, supersedes_revision_id)
VALUES (
    '55555555-5555-5555-5555-555555555559', '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b', 'ACTIVE', '55555555-5555-5555-5555-555555555555'
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555559'
 WHERE id = '11111111-1111-1111-1111-111111111111';
SELECT pg_sleep(18);
COMMIT;
