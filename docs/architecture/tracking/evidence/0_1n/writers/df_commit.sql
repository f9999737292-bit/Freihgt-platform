-- Trigger accepts the revision. Deleting it before COMMIT leaves the deferred FK to fail.
BEGIN;
INSERT INTO transport.transport_execution_revisions (id, execution_id, operating_tenant_id, status)
VALUES (
    '55555555-5555-5555-5555-555555555559',
    '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-00000000000b',
    'SUPERSEDED'
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555559'
 WHERE id = '11111111-1111-1111-1111-111111111111';
DELETE FROM transport.transport_execution_revisions
 WHERE id = '55555555-5555-5555-5555-555555555559';
COMMIT;
