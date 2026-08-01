package repository

const insertStatusHistoryQuery = `
	INSERT INTO transport.shipment_status_history (
		tenant_id, shipment_id, shipment_version,
		from_status, to_status, reason_code, source,
		actor_type, actor_id, correlation_id, occurred_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`

const listStatusHistoryQuery = `
	SELECT id, tenant_id, shipment_id, shipment_version,
		from_status, to_status, reason_code, source,
		actor_type, actor_id, correlation_id, occurred_at, recorded_at
	FROM transport.shipment_status_history
	WHERE tenant_id = $1 AND shipment_id = $2
`

const countStatusHistoryQuery = `
	SELECT COUNT(*)
	FROM transport.shipment_status_history
	WHERE tenant_id = $1 AND shipment_id = $2
`

const hasInitialStatusHistoryQuery = `
	SELECT EXISTS (
		SELECT 1 FROM transport.shipment_status_history
		WHERE tenant_id = $1 AND shipment_id = $2 AND from_status IS NULL
	)
`
