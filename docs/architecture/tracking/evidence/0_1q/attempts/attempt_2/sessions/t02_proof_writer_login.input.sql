-- step 11 [proof_writer_login: SET ROLE proof_issuer_login] sent 2026-11-17T16:13:25.106579+00:00
SET ROLE proof_issuer_login;
-- step 12 [proof_writer_login: SET ROLE proof_authority_owner] sent 2026-11-17T16:13:25.320794+00:00
SET ROLE proof_authority_owner;
-- step 13 [proof_writer_login: SET ROLE proof_fencing_login] sent 2026-11-17T16:13:25.323047+00:00
SET ROLE proof_fencing_login;
-- step 14 [proof_writer_login: SET ROLE proof_fence_owner] sent 2026-11-17T16:13:25.325739+00:00
SET ROLE proof_fence_owner;
-- step 15 [proof_writer_login: SET SESSION AUTHORIZATION proof_issuer_login] sent 2026-11-17T16:13:25.328562+00:00
SET SESSION AUTHORIZATION proof_issuer_login;
-- step 16 [proof_writer_login: identity after denied SET ROLE] sent 2026-11-17T16:13:25.330770+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
