-- Fail closed. Do not delete N-member rows, rewrite their pattern, or copy set counts into pair counts.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM network_optimizer.consolidation_search_runs
        WHERE pattern = 'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER'
    ) OR EXISTS (
        SELECT 1
        FROM network_optimizer.consolidation_candidates
        WHERE pattern = 'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER'
    ) OR EXISTS (
        SELECT 1
        FROM network_optimizer.consolidation_candidate_members
        WHERE ordinal = 3
    ) THEN
        RAISE EXCEPTION 'nlo-0.3e down migration refused: n-member rows or ordinal 3 members exist';
    END IF;
END $$;

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT consolidation_search_runs_count_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP COLUMN evaluated_set_count;

ALTER TABLE network_optimizer.consolidation_search_runs
    ALTER COLUMN evaluated_pair_count SET NOT NULL;

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT consolidation_search_runs_pattern_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_pattern_chk
    CHECK (pattern IN ('SAME_ORIGIN_SAME_DESTINATION', 'CURRENT_TRIP_FILL'));

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT consolidation_search_runs_context_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_context_chk CHECK (
        (pattern = 'SAME_ORIGIN_SAME_DESTINATION' AND capacity_id IS NOT NULL AND capacity_version IS NOT NULL)
        OR (pattern = 'CURRENT_TRIP_FILL' AND capacity_id IS NULL AND capacity_version IS NULL)
    );

ALTER TABLE network_optimizer.consolidation_candidate_members
    DROP CONSTRAINT consolidation_candidate_members_ordinal_chk;

ALTER TABLE network_optimizer.consolidation_candidate_members
    ADD CONSTRAINT consolidation_candidate_members_ordinal_chk
    CHECK (ordinal IN (1, 2));
