-- step 53 [fencing INSERT into proof.tickets] sent 2026-11-17T16:14:01.331067+00:00
INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) VALUES ('ns-x', 'f-1', encode(sha256(convert_to('0.1q:f-1', 'UTF8')), 'hex'), 1, 'proof_issuer_login');
-- step 54 [fencing calls proof.issue_ticket] sent 2026-11-17T16:14:01.515196+00:00
SELECT proof.issue_ticket('ns-x', 'f-2', encode(sha256(convert_to('0.1q:f-2', 'UTF8')), 'hex'), 1);
-- step 55 [fencing ALTER ROLE proof_issuer_login PASSWORD (takeover attempt)] sent 2026-11-17T16:14:01.518190+00:00
\getenv pw PROOF_PW_ATTEMPT
ALTER ROLE proof_issuer_login PASSWORD :'pw';
-- step 56 [fencing ALTER ROLE proof_issuer_login NOLOGIN directly (outside definer)] sent 2026-11-17T16:14:01.521001+00:00
ALTER ROLE proof_issuer_login NOLOGIN;
-- step 57 [fencing GRANT proof_issuer_login TO itself] sent 2026-11-17T16:14:01.523808+00:00
GRANT proof_issuer_login TO proof_fencing_login WITH SET TRUE;
-- step 59 [fencing calls pg_terminate_backend directly on a live other-service backend (no pg_signal_backend)] sent 2026-11-17T16:14:01.742881+00:00
SELECT pg_terminate_backend(299);
