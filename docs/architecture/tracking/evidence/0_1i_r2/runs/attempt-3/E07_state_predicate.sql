SELECT CASE
           WHEN use_status = 'ISSUED' AND consumed_at IS NULL THEN 'PASS'
           ELSE 'FAIL'
       END AS state_predicate
  FROM transport.binding_authority_decisions
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';

