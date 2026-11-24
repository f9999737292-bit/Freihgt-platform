-- step 124 [ib1_revoke_active backend pid] sent 2026-11-17T16:13:05.330770+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 126 [existing backend issues after REVOKE CONNECT] sent 2026-11-17T16:13:05.650355+00:00
SELECT proof.issue_ticket('ns-revoke', 'req-r1', encode(sha256(convert_to('0.1q:req-r1', 'UTF8')), 'hex'), 1);
