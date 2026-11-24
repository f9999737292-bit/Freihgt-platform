-- step 156 [e4_controller_crash backend pid] sent 2026-11-17T16:13:40.742184+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 157 [E4 phase A close] sent 2026-11-17T16:13:40.960923+00:00
SELECT proof.set_handover('ns-e4', 'HANDOVER_PENDING');
-- step 158 [E4 BEGIN transition (uncommitted)] sent 2026-11-17T16:13:41.021682+00:00
BEGIN;
SELECT proof.set_handover('ns-e4', 'QUIESCED');
SELECT proof.transition_epoch('ns-e4', 1, 'proof_issuer_b_login');
-- step 161 [E4 controller COMMIT after crash] sent 2026-11-17T16:13:41.136375+00:00
COMMIT;
