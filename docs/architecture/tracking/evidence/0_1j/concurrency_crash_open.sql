BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(),
       use_status = 'CONSUMED',
       binding_id = 'abababab-abab-abab-abab-abababababab'
 WHERE decision_id = '66666666-6666-6666-6666-666666666678'
   AND use_status = 'ISSUED'
   AND consumed_at IS NULL;
SELECT pg_sleep(30);
COMMIT;
