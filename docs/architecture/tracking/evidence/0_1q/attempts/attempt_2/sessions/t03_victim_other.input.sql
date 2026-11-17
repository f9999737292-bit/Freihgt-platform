-- step 58 [t03_victim_other backend pid] sent 2026-11-17T16:13:31.112668+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
