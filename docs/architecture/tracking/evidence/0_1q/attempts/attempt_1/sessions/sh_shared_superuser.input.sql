-- step 164 [sh_shared_superuser backend pid] sent 2026-11-17T16:13:09.374312+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 165 [shared login direct INSERT stale ticket with forged issued_by] sent 2026-11-17T16:13:09.609415+00:00
INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) VALUES ('ns-main', 'req-shared-direct', repeat('a', 64), 1, 'proof_issuer_login');
-- step 166 [shared login opens the closed gate directly] sent 2026-11-17T16:13:09.617227+00:00
UPDATE proof.handover_state SET gate = 'OPEN' WHERE issuer_namespace_id = 'ns-main';
-- step 167 [shared login assumes issuer identity] sent 2026-11-17T16:13:09.624038+00:00
SET SESSION AUTHORIZATION proof_issuer_login;
SELECT 'VAL=' || session_user || ',' || current_user;
RESET SESSION AUTHORIZATION;
-- step 168 [shared login re-enables old issuer LOGIN] sent 2026-11-17T16:13:09.627424+00:00
ALTER ROLE proof_issuer_login LOGIN;
