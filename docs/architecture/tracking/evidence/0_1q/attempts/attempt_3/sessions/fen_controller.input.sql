-- step 74 [fen_controller backend pid] sent 2026-11-17T16:14:03.224536+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 78 [Phase A: close gates] sent 2026-11-17T16:14:03.243301+00:00
SELECT proof.set_handover('ns-main', 'HANDOVER_PENDING');
SELECT proof.set_handover('ns-n4', 'HANDOVER_PENDING');
-- step 80 [Phase B: deny login for old issuer] sent 2026-11-17T16:14:03.310221+00:00
SELECT proof_fence.deny_login('proof_issuer_login');
-- step 82 [Phase B: record RECONNECT_DENIED] sent 2026-11-17T16:14:03.558854+00:00
SELECT proof.set_handover('ns-main', 'RECONNECT_DENIED');
-- step 92 [Phase C: census by role OID] sent 2026-11-17T16:14:04.722604+00:00
SELECT 'VAL=' || proof_fence.census('proof_issuer_login')::text;
-- step 94 [premature QUIESCED] sent 2026-11-17T16:14:04.731586+00:00
SELECT proof.set_handover('ns-main', 'QUIESCED');
-- step 95 [transition while old backends remain] sent 2026-11-17T16:14:04.739971+00:00
SELECT proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login');
-- step 96 [terminate other-service pid under issuer target] sent 2026-11-17T16:14:04.743842+00:00
SELECT proof_fence.terminate('proof_issuer_login', 353);
-- step 97 [terminate with non-allowlisted target] sent 2026-11-17T16:14:04.747800+00:00
SELECT proof_fence.terminate('proof_other_service_login', 353);
-- step 98 [terminate pid 337] sent 2026-11-17T16:14:04.751699+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 337)::text;
-- step 100 [terminate pid 340] sent 2026-11-17T16:14:04.759515+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 340)::text;
-- step 102 [terminate pid 351] sent 2026-11-17T16:14:04.765687+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 351)::text;
-- step 104 [terminate pid 352] sent 2026-11-17T16:14:04.772541+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 352)::text;
-- step 117 [Phase D: transition epoch] sent 2026-11-17T16:14:04.960878+00:00
SELECT 'VAL=' || (proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login'))::text AS v;
-- step 118 [Phase E: try_open_gate] sent 2026-11-17T16:14:04.967480+00:00
SELECT 'VAL=' || (proof.try_open_gate('ns-main'))::text AS v;
