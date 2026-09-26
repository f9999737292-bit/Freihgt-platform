DROP TABLE IF EXISTS network_optimizer.score_profile_components;
DROP TABLE IF EXISTS network_optimizer.score_profiles;

DROP INDEX IF EXISTS network_optimizer.match_candidates_search_rank_uidx;

ALTER TABLE network_optimizer.match_candidates
    DROP CONSTRAINT IF EXISTS match_candidates_score_status_chk,
    DROP CONSTRAINT IF EXISTS match_candidates_score_total_chk,
    DROP CONSTRAINT IF EXISTS match_candidates_score_evidence_chk,
    DROP CONSTRAINT IF EXISTS match_candidates_rank_chk,
    DROP CONSTRAINT IF EXISTS match_candidates_score_shape_chk,
    DROP COLUMN IF EXISTS rank,
    DROP COLUMN IF EXISTS score_status,
    DROP COLUMN IF EXISTS score_total,
    DROP COLUMN IF EXISTS score_evidence_bps,
    DROP COLUMN IF EXISTS score_fingerprint,
    DROP COLUMN IF EXISTS score_components,
    DROP COLUMN IF EXISTS unranked_reason_codes;

ALTER TABLE network_optimizer.next_load_search_runs
    DROP CONSTRAINT IF EXISTS next_load_search_runs_ranking_currency_chk,
    DROP CONSTRAINT IF EXISTS next_load_search_runs_profile_version_chk,
    DROP COLUMN IF EXISTS score_profile_code,
    DROP COLUMN IF EXISTS score_profile_version,
    DROP COLUMN IF EXISTS score_profile_fingerprint,
    DROP COLUMN IF EXISTS scoring_algorithm_version,
    DROP COLUMN IF EXISTS ranking_currency;

ALTER TABLE network_optimizer.capacity_search_policies
    DROP CONSTRAINT IF EXISTS capacity_search_ranking_currency_chk,
    DROP COLUMN IF EXISTS ranking_currency;

ALTER TABLE network_optimizer.carrier_search_policies
    DROP CONSTRAINT IF EXISTS carrier_search_ranking_currency_chk,
    DROP COLUMN IF EXISTS ranking_currency;
