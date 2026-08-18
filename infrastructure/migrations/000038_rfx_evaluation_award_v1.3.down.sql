DROP INDEX IF EXISTS rfx.idx_rfx_audit_events_entity;
ALTER TABLE rfx.rfx_responses DROP COLUMN IF EXISTS evaluation_rank;
DROP TABLE IF EXISTS rfx.rfx_awards CASCADE;
DROP TABLE IF EXISTS rfx.rfx_response_offer_lines CASCADE;
