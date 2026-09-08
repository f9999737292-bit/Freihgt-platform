DROP INDEX IF EXISTS rfx.idx_rfx_change_impact_expires_unconsumed;
DROP INDEX IF EXISTS rfx.idx_rfx_change_impact_event_candidate_created;

ALTER TABLE rfx.rfx_change_impact_analyses
    DROP CONSTRAINT IF EXISTS fk_rfx_change_impact_source_composite;

ALTER TABLE rfx.rfx_change_impact_analyses
    DROP CONSTRAINT IF EXISTS fk_rfx_change_impact_candidate_composite;

ALTER TABLE rfx.rfx_change_impact_analyses
    DROP CONSTRAINT IF EXISTS fk_rfx_change_impact_event_composite;

DROP INDEX IF EXISTS rfx.uq_rfx_events_tenant_id;

DROP TABLE IF EXISTS rfx.rfx_change_impact_analyses;
