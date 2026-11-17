-- step 134 [e1_naive_epoch_change backend pid] sent 2026-11-17T16:13:06.738694+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 136 [E1 naive epoch change waits for row lock] sent 2026-11-17T16:13:06.947444+00:00
UPDATE proof.issuance_epoch SET epoch = epoch + 1 WHERE issuer_namespace_id = 'ns-e1';
