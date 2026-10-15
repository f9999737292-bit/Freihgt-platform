-- NLO-0.9C-I2A down. Reverse chain persistence only.
-- Does not alter pre-existing route-plan tables beyond the I2A supporting unique constraint.

DROP TABLE IF EXISTS network_optimizer.chain_search_idempotency;
DROP TABLE IF EXISTS network_optimizer.chain_candidates;
DROP TABLE IF EXISTS network_optimizer.chain_search_runs;

ALTER TABLE network_optimizer.route_plans
    DROP CONSTRAINT IF EXISTS route_plans_tenant_id_uidx;
