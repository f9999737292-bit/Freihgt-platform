-- deadlock_timeout is not set here. PostgreSQL 16 rejected SET deadlock_timeout
-- for these roles with SQLSTATE 42501. The runner applies it with ALTER ROLE.
BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(),
       use_status = 'CONSUMED',
       binding_id = 'dededede-dede-dede-dede-dededededede'
 WHERE decision_id = '66666666-6666-6666-6666-666666666670'
   AND use_status = 'ISSUED'
   AND consumed_at IS NULL;
SELECT shipment_id::text
  FROM transport.read_binding_assignment(
      '11111111-1111-1111-1111-111111111111',
      '22222222-2222-2222-2222-222222222222');
COMMIT;
