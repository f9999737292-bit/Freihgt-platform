-- step 49 [other service calls proof.issue_ticket] sent 2026-11-17T16:13:29.576449+00:00
SELECT proof.issue_ticket('ns-x', 'o-1', encode(sha256(convert_to('0.1q:o-1', 'UTF8')), 'hex'), 1);
-- step 50 [other service SELECT proof.tickets] sent 2026-11-17T16:13:29.964146+00:00
SELECT * FROM proof.tickets;
-- step 51 [other service INSERT into its own control table] sent 2026-11-17T16:13:29.983619+00:00
INSERT INTO proof.control_rows (note) VALUES ('other-ok');
-- step 52 [COUNTEREXAMPLE: other service calls proof.unprotected_issue via PUBLIC] sent 2026-11-17T16:13:30.080636+00:00
SELECT proof.unprotected_issue('other-via-public');
