DROP TABLE IF EXISTS network_optimizer.match_candidates;
DROP TABLE IF EXISTS network_optimizer.next_load_search_runs;

ALTER TABLE network_optimizer.capacity_search_policies
    DROP CONSTRAINT IF EXISTS capacity_search_radius_chk,
    DROP COLUMN IF EXISTS radius_km;

ALTER TABLE network_optimizer.carrier_search_policies
    DROP CONSTRAINT IF EXISTS carrier_search_radius_chk,
    DROP COLUMN IF EXISTS radius_km;
