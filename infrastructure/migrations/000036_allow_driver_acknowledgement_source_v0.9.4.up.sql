-- Allow driver-reported critical event acknowledgements to retain source provenance.

ALTER TABLE control_tower.critical_event_acknowledgement
    DROP CONSTRAINT IF EXISTS chk_critical_event_ack_source;

ALTER TABLE control_tower.critical_event_acknowledgement
    ADD CONSTRAINT chk_critical_event_ack_source
        CHECK (source IN ('control-tower', 'driver'));
