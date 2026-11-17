-- step 25 [proof_fencing_direct_login: SET ROLE proof_issuer_b_login] sent 2026-11-17T16:13:59.008323+00:00
SET ROLE proof_issuer_b_login;
-- step 26 [proof_fencing_direct_login: identity after denied SET ROLE] sent 2026-11-17T16:13:59.189991+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
