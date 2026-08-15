-- Driver Mobile Platform v0.1.1: driver.exception_reported outbox rows use
-- driver_reported_exception.id as source_event_id, not shipment_status_history.id.

ALTER TABLE transport.shipment_event_outbox
    DROP CONSTRAINT IF EXISTS fk_shipment_event_outbox_history;
