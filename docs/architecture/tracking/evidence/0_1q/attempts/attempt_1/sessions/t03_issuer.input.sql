-- step 43 [issuer INSERT into proof.tickets directly] sent 2026-11-17T16:12:58.058291+00:00
INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) VALUES ('ns-x', 'i-1', encode(sha256(convert_to('0.1q:i-1', 'UTF8')), 'hex'), 1, 'proof_issuer_login');
-- step 44 [issuer UPDATE proof.issuance_epoch directly] sent 2026-11-17T16:12:58.261795+00:00
UPDATE proof.issuance_epoch SET epoch = epoch + 1;
-- step 45 [issuer SELECT proof.business_rows] sent 2026-11-17T16:12:58.264517+00:00
SELECT * FROM proof.business_rows;
-- step 46 [issuer UPDATE proof.business_rows] sent 2026-11-17T16:12:58.267572+00:00
UPDATE proof.business_rows SET note = 'x';
-- step 47 [issuer calls proof.consume_ticket] sent 2026-11-17T16:12:58.270349+00:00
SELECT proof.consume_ticket(1);
-- step 48 [issuer calls proof_fence.census] sent 2026-11-17T16:12:58.273128+00:00
SELECT proof_fence.census('proof_issuer_login');
