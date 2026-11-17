-- step 11 [proof_writer_login: SET ROLE proof_issuer_login] sent 2026-11-17T16:12:55.259084+00:00
SET ROLE proof_issuer_login;
-- step 12 [proof_writer_login: SET ROLE proof_authority_owner] sent 2026-11-17T16:12:55.479368+00:00
SET ROLE proof_authority_owner;
-- step 13 [proof_writer_login: SET ROLE proof_fencing_login] sent 2026-11-17T16:12:55.482672+00:00
SET ROLE proof_fencing_login;
-- step 14 [proof_writer_login: SET ROLE proof_fence_owner] sent 2026-11-17T16:12:55.487636+00:00
SET ROLE proof_fence_owner;
-- step 15 [proof_writer_login: SET SESSION AUTHORIZATION proof_issuer_login] sent 2026-11-17T16:12:55.490905+00:00
SET SESSION AUTHORIZATION proof_issuer_login;
-- step 16 [proof_writer_login: identity after denied SET ROLE] sent 2026-11-17T16:12:55.494293+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
