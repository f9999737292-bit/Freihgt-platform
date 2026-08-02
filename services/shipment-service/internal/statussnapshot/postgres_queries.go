package statussnapshot

const snapshotCountQueryAll = `
SELECT COUNT(*)::BIGINT, COUNT(DISTINCT tenant_id)::BIGINT
FROM transport.shipments s
WHERE s.deleted_at IS NULL`

const snapshotCountQueryTenant = `
SELECT COUNT(*)::BIGINT, COUNT(DISTINCT tenant_id)::BIGINT
FROM transport.shipments s
WHERE s.deleted_at IS NULL
  AND s.tenant_id = $1`

const snapshotStreamQueryAll = `
WITH ranked_status_history AS (
    SELECT
        h.tenant_id,
        h.shipment_id,
        h.id AS history_id,
        h.shipment_version,
        h.from_status,
        h.to_status,
        h.occurred_at,
        ROW_NUMBER() OVER (
            PARTITION BY h.tenant_id, h.shipment_id
            ORDER BY h.shipment_version DESC, h.occurred_at DESC, h.id DESC
        ) AS rn
    FROM transport.shipment_status_history h
)
SELECT
    s.tenant_id,
    s.id AS shipment_id,
    s.status AS current_status,
    lh.from_status AS previous_status,
    s.version AS aggregate_version,
    o.id AS last_event_id,
    lh.history_id AS last_source_event_id,
    lh.occurred_at AS source_updated_at,
    lh.to_status AS history_to_status,
    lh.shipment_version AS history_version,
    (lh.history_id IS NOT NULL) AS has_history,
    o.aggregate_id AS outbox_aggregate_id,
    o.aggregate_version AS outbox_aggregate_version,
    o.event_type AS outbox_event_type,
    o.status AS outbox_status,
    o.payload AS outbox_payload
FROM transport.shipments s
LEFT JOIN ranked_status_history lh
    ON lh.tenant_id = s.tenant_id
   AND lh.shipment_id = s.id
   AND lh.rn = 1
LEFT JOIN transport.shipment_event_outbox o
    ON o.source_event_id = lh.history_id
WHERE s.deleted_at IS NULL
ORDER BY s.tenant_id, s.id`

const snapshotStreamQueryTenant = `
WITH ranked_status_history AS (
    SELECT
        h.tenant_id,
        h.shipment_id,
        h.id AS history_id,
        h.shipment_version,
        h.from_status,
        h.to_status,
        h.occurred_at,
        ROW_NUMBER() OVER (
            PARTITION BY h.tenant_id, h.shipment_id
            ORDER BY h.shipment_version DESC, h.occurred_at DESC, h.id DESC
        ) AS rn
    FROM transport.shipment_status_history h
    WHERE h.tenant_id = $1
)
SELECT
    s.tenant_id,
    s.id AS shipment_id,
    s.status AS current_status,
    lh.from_status AS previous_status,
    s.version AS aggregate_version,
    o.id AS last_event_id,
    lh.history_id AS last_source_event_id,
    lh.occurred_at AS source_updated_at,
    lh.to_status AS history_to_status,
    lh.shipment_version AS history_version,
    (lh.history_id IS NOT NULL) AS has_history,
    o.aggregate_id AS outbox_aggregate_id,
    o.aggregate_version AS outbox_aggregate_version,
    o.event_type AS outbox_event_type,
    o.status AS outbox_status,
    o.payload AS outbox_payload
FROM transport.shipments s
LEFT JOIN ranked_status_history lh
    ON lh.tenant_id = s.tenant_id
   AND lh.shipment_id = s.id
   AND lh.rn = 1
LEFT JOIN transport.shipment_event_outbox o
    ON o.source_event_id = lh.history_id
WHERE s.deleted_at IS NULL
  AND s.tenant_id = $1
ORDER BY s.tenant_id, s.id`
