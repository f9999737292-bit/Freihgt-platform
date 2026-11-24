-- step 74 [fen_controller backend pid] sent 2026-11-17T16:13:33.110828+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 78 [Phase A: close gates] sent 2026-11-17T16:13:33.136016+00:00
SELECT proof.set_handover('ns-main', 'HANDOVER_PENDING');
SELECT proof.set_handover('ns-n4', 'HANDOVER_PENDING');
-- step 80 [Phase B: deny login for old issuer] sent 2026-11-17T16:13:33.157728+00:00
SELECT proof_fence.deny_login('proof_issuer_login');
-- step 82 [Phase B: record RECONNECT_DENIED] sent 2026-11-17T16:13:33.475671+00:00
SELECT proof.set_handover('ns-main', 'RECONNECT_DENIED');
-- step 92 [Phase C: census by role OID] sent 2026-11-17T16:13:34.909712+00:00
SELECT 'VAL=' || proof_fence.census('proof_issuer_login')::text;
-- step 94 [premature QUIESCED] sent 2026-11-17T16:13:34.921907+00:00
SELECT proof.set_handover('ns-main', 'QUIESCED');
-- step 95 [transition while old backends remain] sent 2026-11-17T16:13:34.976804+00:00
SELECT proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login');
-- step 96 [terminate other-service pid under issuer target] sent 2026-11-17T16:13:34.981767+00:00
SELECT proof_fence.terminate('proof_issuer_login', 348);
-- step 97 [terminate with non-allowlisted target] sent 2026-11-17T16:13:34.986135+00:00
SELECT proof_fence.terminate('proof_other_service_login', 348);
-- step 98 [terminate pid 316] sent 2026-11-17T16:13:34.990066+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 316)::text;
-- step 100 [terminate pid 339] sent 2026-11-17T16:13:34.998076+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 339)::text;
-- step 102 [terminate pid 346] sent 2026-11-17T16:13:35.005447+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 346)::text;
-- step 104 [terminate pid 347] sent 2026-11-17T16:13:35.012533+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 347)::text;
-- step 117 [Phase D: transition epoch] sent 2026-11-17T16:13:35.162414+00:00
SELECT 'VAL=' || (proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login'))::text AS v;
-- step 118 [Phase E: try_open_gate] sent 2026-11-17T16:13:35.170839+00:00
SELECT 'VAL=' || (proof.try_open_gate('ns-main'))::text AS v;
