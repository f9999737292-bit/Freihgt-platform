-- step 134 [e1_naive_epoch_change backend pid] sent 2026-11-17T16:13:39.302827+00:00
SELECT 'VAL=' || (pg_backend_pid())::text AS v;
-- step 136 [E1 naive epoch change waits for row lock] sent 2026-11-17T16:13:39.482073+00:00
UPDATE proof.issuance_epoch SET epoch = epoch + 1 WHERE issuer_namespace_id = 'ns-e1';
