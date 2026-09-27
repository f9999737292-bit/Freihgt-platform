-- NLO-0.3E bounded same-origin N-member search.
-- Pairwise and current-trip rows keep evaluated_pair_count. N-member rows use evaluated_set_count.

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT consolidation_search_runs_pattern_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_pattern_chk
    CHECK (pattern IN (
        'SAME_ORIGIN_SAME_DESTINATION',
        'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER',
        'CURRENT_TRIP_FILL'
    ));

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT consolidation_search_runs_context_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_context_chk CHECK (
        (
            pattern IN ('SAME_ORIGIN_SAME_DESTINATION', 'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER')
            AND capacity_id IS NOT NULL
            AND capacity_version IS NOT NULL
        )
        OR (
            pattern = 'CURRENT_TRIP_FILL'
            AND capacity_id IS NULL
            AND capacity_version IS NULL
        )
    );

ALTER TABLE network_optimizer.consolidation_search_runs
    ALTER COLUMN evaluated_pair_count DROP NOT NULL;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD COLUMN evaluated_set_count integer NULL;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_count_chk CHECK (
        (
            pattern = 'SAME_ORIGIN_SAME_DESTINATION'
            AND evaluated_pair_count IS NOT NULL
            AND evaluated_set_count IS NULL
        )
        OR (
            pattern = 'SAME_ORIGIN_SAME_DESTINATION_N_MEMBER'
            AND evaluated_pair_count IS NULL
            AND evaluated_set_count IS NOT NULL
        )
        OR (
            pattern = 'CURRENT_TRIP_FILL'
            AND evaluated_pair_count IS NOT NULL
            AND evaluated_set_count IS NULL
        )
    );

ALTER TABLE network_optimizer.consolidation_candidate_members
    DROP CONSTRAINT consolidation_candidate_members_ordinal_chk;

ALTER TABLE network_optimizer.consolidation_candidate_members
    ADD CONSTRAINT consolidation_candidate_members_ordinal_chk
    CHECK (ordinal BETWEEN 1 AND 3);
