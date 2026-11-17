-- step 135 [E1 holder BEGIN + issue (locks epoch row)] sent 2026-11-17T16:13:06.938660+00:00
BEGIN;
SELECT proof.issue_ticket('ns-e1', 'req-e1', encode(sha256(convert_to('0.1q:req-e1', 'UTF8')), 'hex'), 1);
-- step 139 [E1 holder COMMIT while epoch change waits] sent 2026-11-17T16:13:07.550610+00:00
COMMIT;
-- step 142 [E2 BEGIN RR + read epoch] sent 2026-11-17T16:13:07.624761+00:00
BEGIN ISOLATION LEVEL REPEATABLE READ;
SELECT epoch FROM proof.issuance_epoch WHERE issuer_namespace_id = 'ns-e2';
-- step 144 [E2 snapshot issue with stale epoch] sent 2026-11-17T16:13:07.635031+00:00
SELECT proof.issue_ticket_snapshot('ns-e2', 'req-e2', encode(sha256(convert_to('0.1q:req-e2', 'UTF8')), 'hex'), 1);
-- step 145 [E2 COMMIT] sent 2026-11-17T16:13:07.639048+00:00
COMMIT;
-- step 148 [E2b BEGIN RR + read epoch] sent 2026-11-17T16:13:07.658575+00:00
BEGIN ISOLATION LEVEL REPEATABLE READ;
SELECT epoch FROM proof.issuance_epoch WHERE issuer_namespace_id = 'ns-e2b';
-- step 150 [E2b FOR UPDATE issue under RR] sent 2026-11-17T16:13:07.668935+00:00
SELECT proof.issue_ticket('ns-e2b', 'req-e2b', encode(sha256(convert_to('0.1q:req-e2b', 'UTF8')), 'hex'), 1);
-- step 151 [E2b ROLLBACK] sent 2026-11-17T16:13:07.671862+00:00
ROLLBACK;
-- step 154 [E2c READ COMMITTED issue with stale expected epoch] sent 2026-11-17T16:13:07.693460+00:00
SELECT proof.issue_ticket('ns-e2c', 'req-e2c', encode(sha256(convert_to('0.1q:req-e2c', 'UTF8')), 'hex'), 1);
