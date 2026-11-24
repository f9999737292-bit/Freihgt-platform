-- step 156 [e4_controller_crash backend pid] sent 2026-11-17T16:14:10.375450+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 157 [E4 phase A close] sent 2026-11-17T16:14:10.576230+00:00
SELECT proof.set_handover('ns-e4', 'HANDOVER_PENDING');
-- step 158 [E4 BEGIN transition (uncommitted)] sent 2026-11-17T16:14:10.632047+00:00
BEGIN;
SELECT proof.set_handover('ns-e4', 'QUIESCED');
SELECT proof.transition_epoch('ns-e4', 1, 'proof_issuer_b_login');
-- step 161 [E4 controller COMMIT after crash] sent 2026-11-17T16:14:10.746793+00:00
COMMIT;
