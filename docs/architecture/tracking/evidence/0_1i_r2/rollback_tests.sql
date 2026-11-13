-- E07. The row-state checks are separate statements after ROLLBACK.
-- The boolean result is saved as psql prints it. The predicate does not compare that text.

-- @admin id=E07_rollback_transaction
BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(),
       use_status = 'CONSUMED'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
ROLLBACK;
-- @end

-- @admin id=E07_use_status expect=OK exact=ISSUED
SELECT use_status
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
-- @end

-- @admin id=E07_consumed_at_is_null expect=OK
SELECT consumed_at IS NULL AS consumed_at_is_null
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
-- @end

-- @admin id=E07_state_predicate expect=OK exact=PASS
SELECT CASE
           WHEN use_status = 'ISSUED' AND consumed_at IS NULL THEN 'PASS'
           ELSE 'FAIL'
       END AS state_predicate
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
-- @end

-- @admin id=HARNESS_multistatement_stdout expect=OK exact=ISSUED|null
SELECT use_status || '|' || CASE WHEN consumed_at IS NULL THEN 'null' ELSE 'set' END AS rolled_back_row
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
-- @end
