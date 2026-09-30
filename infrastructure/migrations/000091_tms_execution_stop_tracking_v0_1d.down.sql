-- Removes only TMS-MSTOP-0.1D tracking objects and restores the prior ETA target shape.

DROP TABLE IF EXISTS tracking.event_outbox;
DROP TABLE IF EXISTS tracking.execution_stop_eta_state;
DROP TABLE IF EXISTS tracking.execution_tracking_state;
DROP TABLE IF EXISTS tracking.execution_event_inbox;

DROP INDEX IF EXISTS tracking.idx_eta_observation_execution_stop;

DELETE FROM tracking.eta_observation WHERE target_type = 'execution_stop';

ALTER TABLE tracking.eta_observation
    DROP CONSTRAINT IF EXISTS chk_eta_observation_target_shape;

ALTER TABLE tracking.eta_observation
    DROP CONSTRAINT IF EXISTS chk_eta_observation_target_type;

ALTER TABLE tracking.eta_observation
    DROP COLUMN IF EXISTS execution_stop_id,
    DROP COLUMN IF EXISTS execution_id;

ALTER TABLE tracking.eta_observation
    ALTER COLUMN shipment_id SET NOT NULL;

ALTER TABLE tracking.eta_observation
    ADD CONSTRAINT chk_eta_observation_target_type
    CHECK (target_type IN ('pickup', 'delivery'));
