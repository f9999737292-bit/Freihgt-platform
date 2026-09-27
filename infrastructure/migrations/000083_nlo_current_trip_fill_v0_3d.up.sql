-- NLO-0.3D current-trip fill reuses the NLO-0.3B consolidation audit tables.
-- Pairwise rows keep a capacity. Current-trip rows are keyed by the search id and have no capacity.

ALTER TABLE network_optimizer.consolidation_search_runs
    DROP CONSTRAINT consolidation_search_runs_pattern_chk;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_pattern_chk
    CHECK (pattern IN ('SAME_ORIGIN_SAME_DESTINATION', 'CURRENT_TRIP_FILL'));

ALTER TABLE network_optimizer.consolidation_search_runs
    ALTER COLUMN capacity_id DROP NOT NULL,
    ALTER COLUMN capacity_version DROP NOT NULL;

ALTER TABLE network_optimizer.consolidation_search_runs
    ADD CONSTRAINT consolidation_search_runs_context_chk CHECK (
        (pattern = 'SAME_ORIGIN_SAME_DESTINATION' AND capacity_id IS NOT NULL AND capacity_version IS NOT NULL)
        OR (pattern = 'CURRENT_TRIP_FILL' AND capacity_id IS NULL AND capacity_version IS NULL)
    );

ALTER TABLE network_optimizer.consolidation_candidates
    DROP CONSTRAINT consolidation_candidates_placement_chk;

ALTER TABLE network_optimizer.consolidation_candidates
    ADD CONSTRAINT consolidation_candidates_placement_chk
    CHECK (placement_check IN ('NOT_EVALUATED', 'SEQUENCE_OK', 'SEQUENCE_CONFLICT', 'REHANDLE_REQUIRED'));

ALTER TABLE network_optimizer.consolidation_candidates
    ALTER COLUMN capacity_id DROP NOT NULL;

ALTER TABLE network_optimizer.consolidation_candidates
    ADD CONSTRAINT consolidation_candidates_fill_capacity_chk CHECK (
        pattern <> 'CURRENT_TRIP_FILL' OR capacity_id IS NULL
    );

ALTER TABLE network_optimizer.consolidation_candidates
    ADD CONSTRAINT consolidation_candidates_run_id_fk
    FOREIGN KEY (search_run_id) REFERENCES network_optimizer.consolidation_search_runs (id);
