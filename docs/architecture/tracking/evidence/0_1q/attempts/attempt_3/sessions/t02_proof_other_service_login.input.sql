-- step 27 [proof_other_service_login: SET ROLE proof_issuer_login] sent 2026-11-17T16:13:59.440731+00:00
SET ROLE proof_issuer_login;
-- step 28 [proof_other_service_login: SET ROLE proof_writer_login] sent 2026-11-17T16:13:59.635750+00:00
SET ROLE proof_writer_login;
-- step 29 [proof_other_service_login: identity after denied SET ROLE] sent 2026-11-17T16:13:59.639484+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
