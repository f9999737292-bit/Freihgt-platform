ALTER TABLE transport.shipment_event_outbox
    ADD CONSTRAINT fk_shipment_event_outbox_history
        FOREIGN KEY (source_event_id)
        REFERENCES transport.shipment_status_history(id)
        ON DELETE CASCADE
    NOT VALID;
