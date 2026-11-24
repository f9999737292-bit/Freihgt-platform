-- step 30 [writer INSERT into proof.tickets] sent 2026-11-17T16:12:57.502343+00:00
INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) VALUES ('ns-x', 'w-1', encode(sha256(convert_to('0.1q:w-1', 'UTF8')), 'hex'), 1, 'proof_issuer_login');
-- step 31 [writer UPDATE proof.tickets status] sent 2026-11-17T16:12:57.683250+00:00
UPDATE proof.tickets SET status = 'ISSUED';
-- step 32 [writer calls proof.issue_ticket] sent 2026-11-17T16:12:57.721905+00:00
SELECT proof.issue_ticket('ns-x', 'w-2', encode(sha256(convert_to('0.1q:w-2', 'UTF8')), 'hex'), 1);
-- step 33 [writer calls proof.issue_ticket_snapshot] sent 2026-11-17T16:12:57.724749+00:00
SELECT proof.issue_ticket_snapshot('ns-x', 'w-3', encode(sha256(convert_to('0.1q:w-3', 'UTF8')), 'hex'), 1);
-- step 34 [writer UPDATE business_rows.owner_epoch (sensitive column)] sent 2026-11-17T16:12:57.728099+00:00
UPDATE proof.business_rows SET owner_epoch = 99 WHERE id = 1;
-- step 35 [writer UPDATE business_rows.note (granted column)] sent 2026-11-17T16:12:57.730875+00:00
UPDATE proof.business_rows SET note = 'writer-ok' WHERE id = 1;
-- step 36 [writer DELETE business_rows] sent 2026-11-17T16:12:57.736925+00:00
DELETE FROM proof.business_rows;
-- step 37 [writer UPDATE proof.issuance_epoch] sent 2026-11-17T16:12:57.739861+00:00
UPDATE proof.issuance_epoch SET epoch = epoch + 1;
-- step 38 [writer calls proof.set_handover] sent 2026-11-17T16:12:57.742457+00:00
SELECT proof.set_handover('ns-x', 'QUIESCED');
-- step 39 [writer calls proof_fence.deny_login] sent 2026-11-17T16:12:57.745141+00:00
SELECT proof_fence.deny_login('proof_issuer_login');
-- step 40 [writer CREATE FUNCTION in schema proof] sent 2026-11-17T16:12:57.748183+00:00
CREATE FUNCTION proof.evil() RETURNS int LANGUAGE sql AS 'SELECT 1';
-- step 41 [writer calls proof.consume_ticket (granted)] sent 2026-11-17T16:12:57.750388+00:00
SELECT proof.consume_ticket(-1);
-- step 42 [COUNTEREXAMPLE: writer calls proof.unprotected_issue (PUBLIC EXECUTE left at default)] sent 2026-11-17T16:12:57.754755+00:00
SELECT proof.unprotected_issue('writer-via-public');
