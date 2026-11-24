BEGIN;
UPDATE transport.transport_execution_revisions
   SET operating_tenant_id = 'dddddddd-dddd-dddd-dddd-dddddddddddd'
 WHERE id = '55555555-5555-5555-5555-555555555555';
UPDATE transport.transport_executions
   SET operating_tenant_id = 'dddddddd-dddd-dddd-dddd-dddddddddddd',
       updated_at = clock_timestamp()
 WHERE id = '11111111-1111-1111-1111-111111111111';
COMMIT;
