-- step 70 [iss3_pool_socket backend pid] sent 2026-11-17T16:13:32.994569+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 109 [iss3_pool_socket client after termination] sent 2026-11-17T16:13:35.076841+00:00
SELECT 1;
