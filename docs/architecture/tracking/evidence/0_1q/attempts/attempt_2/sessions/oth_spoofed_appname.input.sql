-- step 72 [oth_spoofed_appname backend pid] sent 2026-11-17T16:13:33.065502+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 77 [other service BEGIN + insert (left uncommitted)] sent 2026-11-17T16:13:33.131547+00:00
BEGIN;
INSERT INTO proof.control_rows (note) VALUES ('other-in-flight');
-- step 114 [other service COMMIT after handover terminations] sent 2026-11-17T16:13:35.142585+00:00
COMMIT;
