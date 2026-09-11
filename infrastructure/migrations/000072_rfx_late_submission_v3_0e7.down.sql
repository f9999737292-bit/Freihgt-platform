SET search_path = public;

DROP TRIGGER IF EXISTS trg_rfx_late_submission_requests_updated_at ON rfx.rfx_late_submission_requests;

DROP INDEX IF EXISTS rfx.idx_rfx_late_submission_carrier_lookup;
DROP INDEX IF EXISTS rfx.idx_rfx_late_submission_buyer_queue;
DROP INDEX IF EXISTS rfx.uq_rfx_late_submission_active_event_carrier;

ALTER TABLE rfx.rfx_late_submission_requests
    DROP CONSTRAINT IF EXISTS fk_rfx_late_submission_participant_id_composite;
ALTER TABLE rfx.rfx_late_submission_requests
    DROP CONSTRAINT IF EXISTS fk_rfx_late_submission_participant_composite;
ALTER TABLE rfx.rfx_late_submission_requests
    DROP CONSTRAINT IF EXISTS fk_rfx_late_submission_event_composite;

DROP TABLE IF EXISTS rfx.rfx_late_submission_requests;

DROP INDEX IF EXISTS rfx.uq_rfx_participants_tenant_event_id;
DROP INDEX IF EXISTS rfx.uq_rfx_participants_tenant_event_company;
