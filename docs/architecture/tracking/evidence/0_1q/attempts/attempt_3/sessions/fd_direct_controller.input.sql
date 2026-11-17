-- step 173 [direct CREATEROLE controller resets issuer_b password] sent 2026-11-17T16:14:12.808074+00:00
\getenv pw PROOF_PW_TAKEOVER
ALTER ROLE proof_issuer_b_login PASSWORD :'pw';
-- step 175 [direct controller self-grants issuer_b WITH SET TRUE] sent 2026-11-17T16:14:13.297888+00:00
GRANT proof_issuer_b_login TO proof_fencing_direct_login WITH INHERIT TRUE, SET TRUE;
-- step 176 [direct controller SET ROLE issuer_b after self-grant] sent 2026-11-17T16:14:13.357384+00:00
SET ROLE proof_issuer_b_login;
SELECT 'VAL=' || session_user || ',' || current_user;
