-- step 30 [writer INSERT into proof.tickets] sent 2026-11-17T16:13:59.886822+00:00
INSERT INTO proof.tickets (issuer_namespace_id, request_id, payload_hash, issued_epoch, issued_by) VALUES ('ns-x', 'w-1', encode(sha256(convert_to('0.1q:w-1', 'UTF8')), 'hex'), 1, 'proof_issuer_login');
-- step 31 [writer UPDATE proof.tickets status] sent 2026-11-17T16:14:00.071980+00:00
UPDATE proof.tickets SET status = 'ISSUED';
-- step 32 [writer calls proof.issue_ticket] sent 2026-11-17T16:14:00.075277+00:00
SELECT proof.issue_ticket('ns-x', 'w-2', encode(sha256(convert_to('0.1q:w-2', 'UTF8')), 'hex'), 1);
-- step 33 [writer calls proof.issue_ticket_snapshot] sent 2026-11-17T16:14:00.077570+00:00
SELECT proof.issue_ticket_snapshot('ns-x', 'w-3', encode(sha256(convert_to('0.1q:w-3', 'UTF8')), 'hex'), 1);
-- step 34 [writer UPDATE business_rows.owner_epoch (sensitive column)] sent 2026-11-17T16:14:00.080320+00:00
UPDATE proof.business_rows SET owner_epoch = 99 WHERE id = 1;
-- step 35 [writer UPDATE business_rows.note (granted column)] sent 2026-11-17T16:14:00.083034+00:00
UPDATE proof.business_rows SET note = 'writer-ok' WHERE id = 1;
-- step 36 [writer DELETE business_rows] sent 2026-11-17T16:14:00.089960+00:00
DELETE FROM proof.business_rows;
-- step 37 [writer UPDATE proof.issuance_epoch] sent 2026-11-17T16:14:00.093433+00:00
UPDATE proof.issuance_epoch SET epoch = epoch + 1;
-- step 38 [writer calls proof.set_handover] sent 2026-11-17T16:14:00.096718+00:00
SELECT proof.set_handover('ns-x', 'QUIESCED');
-- step 39 [writer calls proof_fence.deny_login] sent 2026-11-17T16:14:00.099511+00:00
SELECT proof_fence.deny_login('proof_issuer_login');
-- step 40 [writer CREATE FUNCTION in schema proof] sent 2026-11-17T16:14:00.103386+00:00
CREATE FUNCTION proof.evil() RETURNS int LANGUAGE sql AS 'SELECT 1';
-- step 41 [writer calls proof.consume_ticket (granted)] sent 2026-11-17T16:14:00.105931+00:00
SELECT proof.consume_ticket(-1);
-- step 42 [COUNTEREXAMPLE: writer calls proof.unprotected_issue (PUBLIC EXECUTE left at default)] sent 2026-11-17T16:14:00.109878+00:00
SELECT proof.unprotected_issue('writer-via-public');
