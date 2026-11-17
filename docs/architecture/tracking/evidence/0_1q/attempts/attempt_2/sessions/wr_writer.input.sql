-- step 73 [wr_writer backend pid] sent 2026-11-17T16:13:33.071320+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 116 [writer still works] sent 2026-11-17T16:13:35.154560+00:00
UPDATE proof.business_rows SET note = 'writer-after-handover' WHERE id = 1;
