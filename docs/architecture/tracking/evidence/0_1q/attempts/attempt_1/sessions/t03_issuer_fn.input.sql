-- step 62 [issuer issue_ticket without payload digest -> ISSUE_DENIED_AUTH] sent 2026-11-17T16:13:00.060629+00:00
SELECT proof.issue_ticket('ns-t03', 'i-nohash', NULL, 1);
-- step 63 [issuer issue_ticket with digest and current epoch (granted path)] sent 2026-11-17T16:13:00.198702+00:00
SELECT proof.issue_ticket('ns-t03', 'i-ok', encode(sha256(convert_to('0.1q:i-ok', 'UTF8')), 'hex'), 1);
-- step 64 [issuer issue_ticket same (ns, request_id) again -> unique violation] sent 2026-11-17T16:13:00.255385+00:00
SELECT proof.issue_ticket('ns-t03', 'i-ok', encode(sha256(convert_to('0.1q:i-ok', 'UTF8')), 'hex'), 1);
