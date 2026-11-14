-- DELETE of the referenced execution. Children use ON DELETE NO ACTION.
BEGIN;
DELETE FROM transport.transport_executions
 WHERE id = '11111111-1111-1111-1111-111111111111';
COMMIT;
