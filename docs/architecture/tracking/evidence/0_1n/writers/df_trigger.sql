-- The revision-mismatch trigger rejects a missing current_revision_id before the deferred FK.
BEGIN;
UPDATE transport.transport_executions
   SET current_revision_id = '55555555-5555-5555-5555-555555555559'
 WHERE id = '11111111-1111-1111-1111-111111111111';
COMMIT;
