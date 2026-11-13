#!/bin/bash
set -u
export PGUSER=proof_admin
export PGDATABASE=binding_privilege_proof_r1
psql -X -q -v ON_ERROR_STOP=1 -c "
BEGIN;
UPDATE transport.binding_authority_decisions
   SET consumed_at = clock_timestamp(), use_status = 'CONSUMED'
 WHERE decision_id = '66666666-6666-6666-6666-666666666666';
ROLLBACK;
"
use_status=$(psql -X -q -tAc "SELECT use_status FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666666';")
consumed_null=$(psql -X -q -tAc "SELECT consumed_at IS NULL FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666666';")
predicate=$(psql -X -q -tAc "SELECT CASE WHEN use_status = 'ISSUED' AND consumed_at IS NULL THEN 'PASS' ELSE 'FAIL' END FROM transport.binding_authority_decisions WHERE decision_id = '66666666-6666-6666-6666-666666666666';")
echo "use_status=${use_status}"
echo "consumed_at_is_null=${consumed_null}"
echo "predicate=${predicate}"
if [ "$use_status" = "ISSUED" ] && [ "$consumed_null" = "t" ] && [ "$predicate" = "PASS" ]; then
  psql -X -q -c "INSERT INTO proof.results(test_name, sqlstate, detail) VALUES ('H5_corrected_field_compare', 'PASS', 'use_status=ISSUED consumed_at_is_null=t predicate=PASS sqlstate=NONE_SUCCESS') ON CONFLICT (test_name) DO UPDATE SET sqlstate = EXCLUDED.sqlstate, detail = EXCLUDED.detail;"
else
  psql -X -q -c "INSERT INTO proof.results(test_name, sqlstate, detail) VALUES ('H5_corrected_field_compare', 'FAIL', 'use_status=${use_status} consumed_at_is_null=${consumed_null} predicate=${predicate}') ON CONFLICT (test_name) DO UPDATE SET sqlstate = EXCLUDED.sqlstate, detail = EXCLUDED.detail;"
  exit 1
fi
