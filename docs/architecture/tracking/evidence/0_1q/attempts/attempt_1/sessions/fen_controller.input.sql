-- step 74 [fen_controller backend pid] sent 2026-11-17T16:13:00.942119+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 78 [Phase A: close gates] sent 2026-11-17T16:13:01.041418+00:00
SELECT proof.set_handover('ns-main', 'HANDOVER_PENDING');
SELECT proof.set_handover('ns-n4', 'HANDOVER_PENDING');
-- step 80 [Phase B: deny login for old issuer] sent 2026-11-17T16:13:01.108583+00:00
SELECT proof_fence.deny_login('proof_issuer_login');
-- step 82 [Phase B: record RECONNECT_DENIED] sent 2026-11-17T16:13:01.327467+00:00
SELECT proof.set_handover('ns-main', 'RECONNECT_DENIED');
-- step 92 [Phase C: census by role OID] sent 2026-11-17T16:13:02.619765+00:00
SELECT 'VAL=' || proof_fence.census('proof_issuer_login')::text;
-- step 94 [premature QUIESCED] sent 2026-11-17T16:13:02.629686+00:00
SELECT proof.set_handover('ns-main', 'QUIESCED');
-- step 95 [transition while old backends remain] sent 2026-11-17T16:13:02.637374+00:00
SELECT proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login');
-- step 96 [terminate other-service pid under issuer target] sent 2026-11-17T16:13:02.642244+00:00
SELECT proof_fence.terminate('proof_issuer_login', 343);
-- step 97 [terminate with non-allowlisted target] sent 2026-11-17T16:13:02.645508+00:00
SELECT proof_fence.terminate('proof_other_service_login', 343);
-- step 98 [terminate pid 333] sent 2026-11-17T16:13:02.648218+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 333)::text;
-- step 100 [terminate pid 334] sent 2026-11-17T16:13:02.655129+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 334)::text;
-- step 102 [terminate pid 335] sent 2026-11-17T16:13:02.661378+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 335)::text;
-- step 104 [terminate pid 336] sent 2026-11-17T16:13:02.667920+00:00
SELECT 'VAL=' || proof_fence.terminate('proof_issuer_login', 336)::text;
-- step 117 [Phase D: transition epoch] sent 2026-11-17T16:13:02.806838+00:00
SELECT 'VAL=' || (proof.transition_epoch('ns-main', 1, 'proof_issuer_b_login'))::text AS v;
-- step 118 [Phase E: try_open_gate] sent 2026-11-17T16:13:02.814023+00:00
SELECT 'VAL=' || (proof.try_open_gate('ns-main'))::text AS v;
