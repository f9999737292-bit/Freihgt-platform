-- step 17 [proof_issuer_login: SET ROLE proof_authority_owner] sent 2026-11-17T16:13:58.113258+00:00
SET ROLE proof_authority_owner;
-- step 18 [proof_issuer_login: SET ROLE proof_writer_login] sent 2026-11-17T16:13:58.301891+00:00
SET ROLE proof_writer_login;
-- step 19 [proof_issuer_login: SET ROLE proof_fence_owner] sent 2026-11-17T16:13:58.305292+00:00
SET ROLE proof_fence_owner;
-- step 20 [proof_issuer_login: identity after denied SET ROLE] sent 2026-11-17T16:13:58.307976+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
