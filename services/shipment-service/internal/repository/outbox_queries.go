package repository

const insertStatusHistoryReturningQuery = `
	INSERT INTO transport.shipment_status_history (
		tenant_id, shipment_id, shipment_version,
		from_status, to_status, reason_code, source,
		actor_type, actor_id, correlation_id, occurred_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	RETURNING id, tenant_id, shipment_id, shipment_version,
		from_status, to_status, reason_code, source,
		actor_type, actor_id, correlation_id, occurred_at, recorded_at
`

const insertOutboxEventQuery = `
	INSERT INTO transport.shipment_event_outbox (
		id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
		event_type, schema_version, source_event_id, payload, headers,
		status, attempts, available_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
`

const claimPendingOutboxQuery = `
	SELECT id, tenant_id, aggregate_type, aggregate_id, aggregate_version,
		event_type, schema_version, source_event_id, payload, headers,
		status, attempts, available_at, locked_at, locked_by,
		published_at, last_error_code, created_at
	FROM transport.shipment_event_outbox
	WHERE status = 'PENDING'
	  AND available_at <= $1
	  AND (locked_at IS NULL OR locked_at < $2)
	ORDER BY created_at ASC, id ASC
	FOR UPDATE SKIP LOCKED
	LIMIT $3
`

const lockClaimedOutboxQuery = `
	UPDATE transport.shipment_event_outbox
	SET locked_at = $1,
	    locked_by = $2,
	    attempts = attempts + 1
	WHERE id = $3
	  AND status = 'PENDING'
`

const markOutboxPublishedQuery = `
	UPDATE transport.shipment_event_outbox
	SET status = 'PUBLISHED',
	    published_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = NULL
	WHERE id = $2
	  AND status = 'PENDING'
	  AND locked_by = $3
`

const releaseOutboxWithRetryQuery = `
	UPDATE transport.shipment_event_outbox
	SET status = 'PENDING',
	    available_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = $2
	WHERE id = $3
	  AND status = 'PENDING'
	  AND locked_by = $4
`

const markOutboxFailedQuery = `
	UPDATE transport.shipment_event_outbox
	SET status = 'FAILED',
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = $1
	WHERE id = $2
	  AND status = 'PENDING'
	  AND locked_by = $3
`

const countOutboxPendingQuery = `
	SELECT COUNT(*)
	FROM transport.shipment_event_outbox
	WHERE status = 'PENDING'
`

const countOutboxFailedQuery = `
	SELECT COUNT(*)
	FROM transport.shipment_event_outbox
	WHERE status = 'FAILED'
`

const oldestPendingOutboxAgeQuery = `
	SELECT COALESCE(EXTRACT(EPOCH FROM ($1 - MIN(created_at))), 0)
	FROM transport.shipment_event_outbox
	WHERE status = 'PENDING'
`

const listFailedOutboxForReplayQuery = `
	SELECT id, tenant_id, aggregate_id, event_type, status, attempts, last_error_code
	FROM transport.shipment_event_outbox
	WHERE tenant_id = $1
	  AND status = 'FAILED'
	  AND ($2::uuid[] IS NULL OR id = ANY($2::uuid[]))
	  AND ($3::uuid[] IS NULL OR aggregate_id = ANY($3::uuid[]))
	  AND (
	    ($2::uuid[] IS NOT NULL AND cardinality($2::uuid[]) > 0)
	    OR ($3::uuid[] IS NOT NULL AND cardinality($3::uuid[]) > 0)
	  )
	ORDER BY aggregate_id ASC, created_at ASC, id ASC
`

const listOutboxReplayOrderingQuery = `
	SELECT id, status, created_at, aggregate_version
	FROM transport.shipment_event_outbox
	WHERE tenant_id = $1
	  AND aggregate_id = $2
	ORDER BY created_at ASC, id ASC
`

const replayFailedOutboxRowsQuery = `
	UPDATE transport.shipment_event_outbox
	SET status = 'PENDING',
	    attempts = 0,
	    available_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = NULL,
	    published_at = NULL
	WHERE tenant_id = $2
	  AND status = 'FAILED'
	  AND id = ANY($3::uuid[])
`
