package repository

const insertFreightCostOutboxEventQuery = `
	INSERT INTO billing.freight_cost_outbox (
		id, tenant_id, aggregate_type, aggregate_id, source_revision,
		event_type, schema_version, payload,
		status, attempts, available_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`

const claimPendingFreightCostOutboxQuery = `
	SELECT id, tenant_id, aggregate_type, aggregate_id, source_revision,
		event_type, schema_version, payload,
		status, attempts, available_at, locked_at, locked_by,
		published_at, last_error_code, created_at
	FROM billing.freight_cost_outbox
	WHERE status = 'PENDING'
	  AND available_at <= $1
	  AND (locked_at IS NULL OR locked_at < $2)
	ORDER BY created_at ASC, id ASC
	FOR UPDATE SKIP LOCKED
	LIMIT $3
`

const lockClaimedFreightCostOutboxQuery = `
	UPDATE billing.freight_cost_outbox
	SET locked_at = $1,
	    locked_by = $2,
	    attempts = attempts + 1
	WHERE id = $3
	  AND status = 'PENDING'
`

const markFreightCostOutboxPublishedQuery = `
	UPDATE billing.freight_cost_outbox
	SET status = 'PUBLISHED',
	    published_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = NULL
	WHERE id = $2
	  AND status = 'PENDING'
	  AND locked_by = $3
`

const releaseFreightCostOutboxWithRetryQuery = `
	UPDATE billing.freight_cost_outbox
	SET status = 'PENDING',
	    available_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = $2
	WHERE id = $3
	  AND status = 'PENDING'
	  AND locked_by = $4
`

const markFreightCostOutboxFailedQuery = `
	UPDATE billing.freight_cost_outbox
	SET status = 'FAILED',
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = $1
	WHERE id = $2
	  AND status = 'PENDING'
	  AND locked_by = $3
`

const countFreightCostOutboxPendingQuery = `
	SELECT COUNT(*)
	FROM billing.freight_cost_outbox
	WHERE status = 'PENDING'
`

const countFreightCostOutboxFailedQuery = `
	SELECT COUNT(*)
	FROM billing.freight_cost_outbox
	WHERE status = 'FAILED'
`

const oldestPendingFreightCostOutboxAgeQuery = `
	SELECT COALESCE(EXTRACT(EPOCH FROM ($1 - MIN(created_at))), 0)
	FROM billing.freight_cost_outbox
	WHERE status = 'PENDING'
`

const freightSettlementSelectColumns = `
	id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
	award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
	approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
	status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
	idempotency_key, version, billing_link_revision, created_at, created_by, updated_at, rate_snapshot_id, pricing_source`
