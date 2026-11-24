-- New execution. Does not lock execution 1111. The revision FK key-shares the new row.
BEGIN;
INSERT INTO transport.transport_executions (id, operating_tenant_id, current_revision_id)
VALUES ('12121212-1212-1212-1212-121212121212', '00000000-0000-0000-0000-00000000000b', NULL);
INSERT INTO transport.transport_execution_revisions (id, execution_id, operating_tenant_id, status)
VALUES (
    '55555555-5555-5555-5555-555555555558',
    '12121212-1212-1212-1212-121212121212',
    '00000000-0000-0000-0000-00000000000b',
    'ACTIVE'
);
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555558'
 WHERE id = '12121212-1212-1212-1212-121212121212';
COMMIT;
