package repository

const insertPaymentOutboxEventQuery = `
	INSERT INTO billing.payment_outbox (
		id, tenant_id, aggregate_type, aggregate_id,
		event_type, schema_version, payload,
		status, attempts, available_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`

const claimPendingPaymentOutboxQuery = `
	SELECT id, tenant_id, aggregate_type, aggregate_id,
		event_type, schema_version, payload,
		status, attempts, available_at, locked_at, locked_by,
		published_at, last_error_code, created_at
	FROM billing.payment_outbox
	WHERE status = 'PENDING'
	  AND available_at <= $1
	  AND (locked_at IS NULL OR locked_at < $2)
	ORDER BY created_at ASC, id ASC
	FOR UPDATE SKIP LOCKED
	LIMIT $3
`

const lockClaimedPaymentOutboxQuery = `
	UPDATE billing.payment_outbox
	SET locked_at = $1,
	    locked_by = $2,
	    attempts = attempts + 1
	WHERE id = $3
	  AND status = 'PENDING'
`

const markPaymentOutboxPublishedQuery = `
	UPDATE billing.payment_outbox
	SET status = 'PUBLISHED',
	    published_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = NULL
	WHERE id = $2
	  AND status = 'PENDING'
	  AND locked_by = $3
`

const markPaymentOutboxPublishedByAggregateQuery = `
	UPDATE billing.payment_outbox
	SET status = 'PUBLISHED',
	    published_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = NULL
	WHERE tenant_id = $2
	  AND event_type = $3
	  AND aggregate_id = $4
	  AND status = 'PENDING'
`

const releasePaymentOutboxWithRetryQuery = `
	UPDATE billing.payment_outbox
	SET status = 'PENDING',
	    available_at = $1,
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = $2
	WHERE id = $3
	  AND status = 'PENDING'
	  AND locked_by = $4
`

const markPaymentOutboxFailedQuery = `
	UPDATE billing.payment_outbox
	SET status = 'FAILED',
	    locked_at = NULL,
	    locked_by = NULL,
	    last_error_code = $1
	WHERE id = $2
	  AND status = 'PENDING'
	  AND locked_by = $3
`

const countPaymentOutboxPendingQuery = `
	SELECT COUNT(*)
	FROM billing.payment_outbox
	WHERE status = 'PENDING'
`

const countPaymentOutboxFailedQuery = `
	SELECT COUNT(*)
	FROM billing.payment_outbox
	WHERE status = 'FAILED'
`

const oldestPendingPaymentOutboxAgeQuery = `
	SELECT COALESCE(EXTRACT(EPOCH FROM ($1 - MIN(created_at))), 0)
	FROM billing.payment_outbox
	WHERE status = 'PENDING'
`

const paymentOutboxIdempotencyConstraint = "uq_payment_outbox_tenant_event_aggregate"
