DELETE FROM network_optimizer.consolidation_candidate_members AS member
USING network_optimizer.consolidation_candidates AS candidate
WHERE member.candidate_id = candidate.id
  AND candidate.pattern = 'CURRENT_TRIP_FILL';

DELETE FROM network_optimizer.consolidation_candidates
WHERE pattern = 'CURRENT_TRIP_FILL';

DELETE FROM network_optimizer.consolidation_search_runs
WHERE pattern = 'CURRENT_TRIP_FILL';

ALTER TABLE network_optimizer.consolidation_candidates
    DROP CONSTRAINT IF EXISTS consolidation_candidates_run_id_fk;

ALTER TABLE network_optimizer.consolidation_candidates
    DROP CONSTRAINT IF EXISTS consolidation_candidates_fill_capacity_chk;

ALTER TABLE network_optimizer.consolidation_candidates
    ALTER COLUMN capacity_id SET NOT NULL;

ALTER TABLE network_optimizer.consolidation_candidates
    DROP CONSTRAINT consolidation_candidates_placement_chk;

ALTER TABLE network_optimizer.consolidation_candidates
    ADD CONSTRAINT consolidation_candidates_placement_chk
    CHECK (placement_check = 'NOT_EVALUATED');

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT IF EXISTS consolidation_search_runs_context_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    ALTER COLUMN capacity_id SET NOT NULL,
    ALTER COLUMN capacity_version SET NOT NULL;

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT consolidation_search_runs_pattern_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_pattern_chk
    CHECK (pattern = 'SAME_ORIGIN_SAME_DESTINATION');
