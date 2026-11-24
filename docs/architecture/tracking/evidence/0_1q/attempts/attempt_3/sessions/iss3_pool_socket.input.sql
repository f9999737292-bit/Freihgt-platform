-- step 70 [iss3_pool_socket backend pid] sent 2026-11-17T16:14:03.135859+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 109 [iss3_pool_socket client after termination] sent 2026-11-17T16:14:04.833333+00:00
SELECT 1;
