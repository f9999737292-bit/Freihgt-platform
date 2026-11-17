-- step 11 [proof_writer_login: SET ROLE proof_issuer_login] sent 2026-11-17T16:13:57.684500+00:00
SET ROLE proof_issuer_login;
-- step 12 [proof_writer_login: SET ROLE proof_authority_owner] sent 2026-11-17T16:13:57.855738+00:00
SET ROLE proof_authority_owner;
-- step 13 [proof_writer_login: SET ROLE proof_fencing_login] sent 2026-11-17T16:13:57.858749+00:00
SET ROLE proof_fencing_login;
-- step 14 [proof_writer_login: SET ROLE proof_fence_owner] sent 2026-11-17T16:13:57.860966+00:00
SET ROLE proof_fence_owner;
-- step 15 [proof_writer_login: SET SESSION AUTHORIZATION proof_issuer_login] sent 2026-11-17T16:13:57.863175+00:00
SET SESSION AUTHORIZATION proof_issuer_login;
-- step 16 [proof_writer_login: identity after denied SET ROLE] sent 2026-11-17T16:13:57.865292+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
