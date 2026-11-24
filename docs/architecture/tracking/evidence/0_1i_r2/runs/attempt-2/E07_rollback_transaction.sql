BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(),
       use_status = 'CONSUMED'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
ROLLBACK;

