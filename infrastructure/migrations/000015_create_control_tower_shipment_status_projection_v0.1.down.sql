DROP INDEX IF EXISTS control_tower.idx_shipment_status_projection_tenant_updated;
DROP INDEX IF EXISTS control_tower.idx_shipment_status_projection_tenant_status;

DROP TABLE IF EXISTS control_tower.shipment_status_event_dead_letter;
DROP TABLE IF EXISTS control_tower.shipment_status_projection;
DROP TABLE IF EXISTS control_tower.shipment_status_event_inbox;

DROP SCHEMA IF EXISTS control_tower;
