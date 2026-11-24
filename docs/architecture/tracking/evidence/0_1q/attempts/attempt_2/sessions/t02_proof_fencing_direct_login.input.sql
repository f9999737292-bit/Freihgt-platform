-- step 25 [proof_fencing_direct_login: SET ROLE proof_issuer_b_login] sent 2026-11-17T16:13:26.825257+00:00
SET ROLE proof_issuer_b_login;
-- step 26 [proof_fencing_direct_login: identity after denied SET ROLE] sent 2026-11-17T16:13:27.051393+00:00
SELECT 'VAL=' || (session_user || ',' || current_user)::text AS v;
