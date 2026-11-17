-- step 69 [iss2_pool_idle backend pid] sent 2026-11-17T16:13:32.839288+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 79 [old issuer issues after gate close] sent 2026-11-17T16:13:33.151290+00:00
SELECT proof.issue_ticket('ns-main', 'req-after-close', encode(sha256(convert_to('0.1q:req-after-close', 'UTF8')), 'hex'), 1);
-- step 108 [iss2_pool_idle client after termination] sent 2026-11-17T16:13:35.049684+00:00
SELECT 1;
