-- step 21 [proof_fencing_login: SET ROLE proof_issuer_login] sent 2026-11-17T16:13:58.555647+00:00
SET ROLE proof_issuer_login;
-- step 22 [proof_fencing_login: SET ROLE proof_fence_owner] sent 2026-11-17T16:13:58.753694+00:00
SET ROLE proof_fence_owner;
-- step 23 [proof_fencing_login: SET ROLE proof_authority_owner] sent 2026-11-17T16:13:58.756434+00:00
SET ROLE proof_authority_owner;
-- step 24 [proof_fencing_login: identity after denied SET ROLE] sent 2026-11-17T16:13:58.758899+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
