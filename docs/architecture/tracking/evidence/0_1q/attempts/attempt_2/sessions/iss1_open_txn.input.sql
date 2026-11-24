-- step 68 [iss1_open_txn backend pid] sent 2026-11-17T16:13:32.622752+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 75 [iss1 BEGIN + issue ticket req-open-1 (left uncommitted)] sent 2026-11-17T16:13:33.114696+00:00
BEGIN;
SELECT proof.issue_ticket('ns-main', 'req-open-1', encode(sha256(convert_to('0.1q:req-open-1', 'UTF8')), 'hex'), 1);
-- step 83 [iss1 still alive inside its transaction after ban] sent 2026-11-17T16:13:33.484084+00:00
SELECT 'VAL=' || txid_current_if_assigned()::text;
-- step 107 [iss1_open_txn client after termination] sent 2026-11-17T16:13:35.023034+00:00
COMMIT;
