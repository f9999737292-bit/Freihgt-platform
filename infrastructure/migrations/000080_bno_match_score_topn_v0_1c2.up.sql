-- BNO-0.1C2 deterministic match score, objective profiles, and top-N.
-- Hard feasibility stays on eligibility. Score is never a substitute for rejection.

ALTER TABLE network_optimizer.carrier_search_policies
    ADD COLUMN ranking_currency text,
    ADD CONSTRAINT carrier_search_ranking_currency_chk CHECK (ranking_currency IS NULL OR ranking_currency ~ '^[A-Z]{3}$');

ALTER TABLE network_optimizer.capacity_search_policies
    ADD COLUMN ranking_currency text,
    ADD CONSTRAINT capacity_search_ranking_currency_chk CHECK (ranking_currency IS NULL OR ranking_currency ~ '^[A-Z]{3}$');

ALTER TABLE network_optimizer.next_load_search_runs
    ADD COLUMN score_profile_code text,
    ADD COLUMN score_profile_version integer,
    ADD COLUMN score_profile_fingerprint text,
    ADD COLUMN scoring_algorithm_version text,
    ADD COLUMN ranking_currency text,
    ADD CONSTRAINT next_load_search_runs_ranking_currency_chk CHECK (ranking_currency IS NULL OR ranking_currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT next_load_search_runs_profile_version_chk CHECK (score_profile_version IS NULL OR score_profile_version > 0);

ALTER TABLE network_optimizer.match_candidates
    ADD COLUMN rank integer,
    ADD COLUMN score_status text NOT NULL DEFAULT 'NOT_APPLICABLE',
    ADD COLUMN score_total integer,
    ADD COLUMN score_evidence_bps integer,
    ADD COLUMN score_fingerprint text,
    ADD COLUMN score_components jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN unranked_reason_codes text[] NOT NULL DEFAULT '{}';

UPDATE network_optimizer.match_candidates
SET score_status = 'UNRANKED',
    unranked_reason_codes = ARRAY['SCORE_NOT_RECORDED']
WHERE eligibility = 'ELIGIBLE';

ALTER TABLE network_optimizer.match_candidates
    ADD CONSTRAINT match_candidates_score_status_chk CHECK (score_status IN ('NOT_APPLICABLE', 'RANKED', 'UNRANKED')),
    ADD CONSTRAINT match_candidates_score_total_chk CHECK (score_total IS NULL OR (score_total >= 0 AND score_total <= 10000)),
    ADD CONSTRAINT match_candidates_score_evidence_chk CHECK (score_evidence_bps IS NULL OR (score_evidence_bps >= 0 AND score_evidence_bps <= 10000)),
    ADD CONSTRAINT match_candidates_rank_chk CHECK (rank IS NULL OR rank > 0),
    ADD CONSTRAINT match_candidates_score_shape_chk CHECK (
        (eligibility = 'REJECTED' AND score_status = 'NOT_APPLICABLE' AND rank IS NULL AND score_total IS NULL)
        OR (eligibility = 'ELIGIBLE' AND score_status = 'RANKED' AND rank IS NOT NULL AND score_total IS NOT NULL AND score_evidence_bps IS NOT NULL)
        OR (eligibility = 'ELIGIBLE' AND score_status = 'UNRANKED' AND rank IS NULL AND score_total IS NULL)
    );

CREATE UNIQUE INDEX match_candidates_search_rank_uidx
    ON network_optimizer.match_candidates (search_run_id, rank)
    WHERE rank IS NOT NULL;

CREATE TABLE network_optimizer.score_profiles (
    id uuid PRIMARY KEY,
    code text NOT NULL CHECK (code <> ''),
    scope text NOT NULL CHECK (scope = 'SYSTEM'),
    tenant_id uuid,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('ACTIVE', 'RESERVED')),
    algorithm_version text NOT NULL CHECK (algorithm_version <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT score_profiles_system_tenant_chk CHECK (scope <> 'SYSTEM' OR tenant_id IS NULL),
    CONSTRAINT score_profiles_code_version_uidx UNIQUE (code, version)
);

CREATE UNIQUE INDEX score_profiles_active_system_uidx
    ON network_optimizer.score_profiles (code)
    WHERE status = 'ACTIVE' AND scope = 'SYSTEM';

CREATE TABLE network_optimizer.score_profile_components (
    profile_id uuid NOT NULL REFERENCES network_optimizer.score_profiles (id) ON DELETE CASCADE,
    component_code text NOT NULL CHECK (component_code <> ''),
    weight_bps integer NOT NULL CHECK (weight_bps > 0),
    required boolean NOT NULL,
    ordinal integer NOT NULL CHECK (ordinal > 0),
    PRIMARY KEY (profile_id, component_code)
);

INSERT INTO network_optimizer.score_profiles (id, code, scope, tenant_id, version, status, algorithm_version)
VALUES
    ('00000000-0000-4000-8000-000000000201', 'MIN_DEADHEAD', 'SYSTEM', NULL, 1, 'ACTIVE', 'bno-score-0.1c2.1'),
    ('00000000-0000-4000-8000-000000000202', 'MAX_CAPACITY_UTILIZATION', 'SYSTEM', NULL, 1, 'ACTIVE', 'bno-score-0.1c2.1'),
    ('00000000-0000-4000-8000-000000000203', 'RETURN_HOME', 'SYSTEM', NULL, 1, 'ACTIVE', 'bno-score-0.1c2.1'),
    ('00000000-0000-4000-8000-000000000204', 'MAX_REVENUE', 'SYSTEM', NULL, 1, 'ACTIVE', 'bno-score-0.1c2.1'),
    ('00000000-0000-4000-8000-000000000205', 'MIN_RISK', 'SYSTEM', NULL, 1, 'ACTIVE', 'bno-score-0.1c2.1'),
    ('00000000-0000-4000-8000-000000000206', 'BALANCED', 'SYSTEM', NULL, 1, 'ACTIVE', 'bno-score-0.1c2.1'),
    ('00000000-0000-4000-8000-000000000207', 'MAX_CONTRIBUTION', 'SYSTEM', NULL, 1, 'RESERVED', 'bno-score-0.1c2.1');

INSERT INTO network_optimizer.score_profile_components (profile_id, component_code, weight_bps, required, ordinal)
VALUES
    ('00000000-0000-4000-8000-000000000201', 'DEADHEAD_EFFICIENCY', 10000, true, 1),
    ('00000000-0000-4000-8000-000000000202', 'CAPACITY_UTILIZATION', 10000, true, 1),
    ('00000000-0000-4000-8000-000000000203', 'TARGET_PROXIMITY', 10000, true, 1),
    ('00000000-0000-4000-8000-000000000204', 'REVENUE', 10000, true, 1),
    ('00000000-0000-4000-8000-000000000205', 'PICKUP_SLACK', 5000, true, 1),
    ('00000000-0000-4000-8000-000000000205', 'PREDICTION_CONFIDENCE', 3000, false, 2),
    ('00000000-0000-4000-8000-000000000205', 'ETA_UNCERTAINTY', 2000, false, 3),
    ('00000000-0000-4000-8000-000000000206', 'DEADHEAD_EFFICIENCY', 3500, true, 1),
    ('00000000-0000-4000-8000-000000000206', 'CAPACITY_UTILIZATION', 2500, false, 2),
    ('00000000-0000-4000-8000-000000000206', 'WAITING_EFFICIENCY', 1500, false, 3),
    ('00000000-0000-4000-8000-000000000206', 'TARGET_PROXIMITY', 1000, false, 4),
    ('00000000-0000-4000-8000-000000000206', 'PICKUP_SLACK', 1000, false, 5),
    ('00000000-0000-4000-8000-000000000206', 'NETWORK_VALUE', 500, false, 6);
