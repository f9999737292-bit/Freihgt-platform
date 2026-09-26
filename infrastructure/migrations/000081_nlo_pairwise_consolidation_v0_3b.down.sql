DROP TABLE IF EXISTS network_optimizer.consolidation_candidate_members;
DROP TABLE IF EXISTS network_optimizer.consolidation_candidates;
DROP TABLE IF EXISTS network_optimizer.consolidation_search_runs;

DROP INDEX IF EXISTS network_optimizer.load_opportunities_published_opt_in_od_idx;

ALTER TABLE network_optimizer.load_opportunities
    DROP COLUMN IF EXISTS cross_shipper_consolidation_allowed,
    DROP COLUMN IF EXISTS consolidation_allowed;
