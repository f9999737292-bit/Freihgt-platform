-- step 71 [iss4_inflight_commit backend pid] sent 2026-11-17T16:13:00.895187+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 76 [iss4 BEGIN + issue ticket req-n4 (left uncommitted)] sent 2026-11-17T16:13:01.032562+00:00
BEGIN;
SELECT proof.issue_ticket('ns-n4', 'req-n4', encode(sha256(convert_to('0.1q:req-n4', 'UTF8')), 'hex'), 1);
-- step 85 [iss4 COMMIT after close + deny] sent 2026-11-17T16:13:01.389580+00:00
COMMIT;
-- step 110 [iss4_inflight_commit client after termination] sent 2026-11-17T16:13:02.754184+00:00
SELECT 1;
