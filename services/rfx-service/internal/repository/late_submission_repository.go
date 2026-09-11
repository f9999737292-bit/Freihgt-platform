package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type LateSubmissionRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
	// injectStoreFailure is set only by integration tests to verify transactional rollback.
	injectStoreFailure bool
}

func NewLateSubmissionRepository(pool *pgxpool.Pool) *LateSubmissionRepository {
	return &LateSubmissionRepository{pool: pool}
}

func (r *LateSubmissionRepository) SetInjectStoreFailure(enabled bool) {
	r.injectStoreFailure = enabled
}

func (r *LateSubmissionRepository) WithTx(tx pgx.Tx) *LateSubmissionRepository {
	return &LateSubmissionRepository{pool: r.pool, exec: tx, injectStoreFailure: r.injectStoreFailure}
}

func (r *LateSubmissionRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

const lateSubmissionSelectColumns = `
	id, tenant_id, rfx_event_id, carrier_company_id, participant_id,
	reason_code, reason_text, requested_until, status,
	approved_valid_from, approved_valid_until, decision_comment,
	requested_by, decided_by, decided_at, consumed_at, version,
	created_at, updated_at
`

func (r *LateSubmissionRepository) CreateRequest(ctx context.Context, req domain.LateSubmissionRequest) (*domain.LateSubmissionRequest, error) {
	if r.injectStoreFailure {
		return nil, mapDBError(fmt.Errorf("injected late submission store failure"))
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_late_submission_requests (
			tenant_id, rfx_event_id, carrier_company_id, participant_id,
			reason_code, reason_text, requested_until, status, requested_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+lateSubmissionSelectColumns,
		req.TenantID, req.RfxEventID, req.CarrierCompanyID, req.ParticipantID,
		string(req.ReasonCode), req.ReasonText, req.RequestedUntil.UTC(), string(domain.LateSubmissionStatusRequested), req.RequestedBy,
	)
	return scanLateSubmissionRequest(row)
}

func (r *LateSubmissionRepository) GetByID(ctx context.Context, id, tenantID, eventID uuid.UUID) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3
	`, id, tenantID, eventID)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("late submission request not found")
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) GetOwnByEventAndCarrier(ctx context.Context, tenantID, eventID, carrierCompanyID uuid.UUID) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, eventID, carrierCompanyID)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("late submission request not found")
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) ListOwnByEventAndCarrier(ctx context.Context, tenantID, eventID, carrierCompanyID uuid.UUID) ([]domain.LateSubmissionRequest, error) {
	rows, err := r.db().Query(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3
		ORDER BY created_at DESC
	`, tenantID, eventID, carrierCompanyID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanLateSubmissionRequests(rows)
}

func (r *LateSubmissionRepository) ListByEvent(ctx context.Context, tenantID, eventID uuid.UUID) ([]domain.LateSubmissionRequest, error) {
	rows, err := r.db().Query(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2
		ORDER BY created_at DESC
	`, tenantID, eventID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanLateSubmissionRequests(rows)
}

func (r *LateSubmissionRepository) GetActiveByEventAndCarrier(ctx context.Context, tenantID, eventID, carrierCompanyID uuid.UUID) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3
			AND status IN ('REQUESTED', 'APPROVED')
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, eventID, carrierCompanyID)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) LockActiveByEventAndCarrierForUpdate(ctx context.Context, tenantID, eventID, carrierCompanyID uuid.UUID) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND carrier_company_id = $3
			AND status IN ('REQUESTED', 'APPROVED')
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, tenantID, eventID, carrierCompanyID)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) MaterializeExpired(
	ctx context.Context,
	id, tenantID, eventID, carrierCompanyID uuid.UUID,
	expectedVersion int,
	expiredAt time.Time,
) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_late_submission_requests SET
			status = 'EXPIRED',
			approved_valid_from = NULL,
			approved_valid_until = NULL,
			version = version + 1,
			updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3 AND carrier_company_id = $4
			AND status = 'APPROVED' AND approved_valid_until <= $5 AND version = $6
		RETURNING `+lateSubmissionSelectColumns,
		id, tenantID, eventID, carrierCompanyID, expiredAt.UTC(), expectedVersion,
	)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("late submission permission cannot be expired", map[string]any{"field": "status"})
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) LockForDecision(ctx context.Context, id, tenantID, eventID uuid.UUID) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3
		FOR UPDATE
	`, id, tenantID, eventID)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("late submission request not found")
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) LockApprovedForConsume(ctx context.Context, id, tenantID, eventID, carrierCompanyID uuid.UUID) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+lateSubmissionSelectColumns+`
		FROM rfx.rfx_late_submission_requests
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3 AND carrier_company_id = $4 AND status = 'APPROVED'
		FOR UPDATE
	`, id, tenantID, eventID, carrierCompanyID)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("approved late submission permission not found")
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) Approve(ctx context.Context, id, tenantID, eventID uuid.UUID, expectedVersion int, validFrom, validUntil time.Time, decidedBy uuid.UUID, comment *string) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_late_submission_requests SET
			status = 'APPROVED',
			approved_valid_from = $4,
			approved_valid_until = $5,
			decision_comment = $6,
			decided_by = $7,
			decided_at = now(),
			version = version + 1,
			updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3 AND status = 'REQUESTED' AND version = $8
		RETURNING `+lateSubmissionSelectColumns,
		id, tenantID, eventID, validFrom.UTC(), validUntil.UTC(), comment, decidedBy, expectedVersion,
	)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("late submission request state conflict", map[string]any{"field": "expected_version"})
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) Reject(ctx context.Context, id, tenantID, eventID uuid.UUID, expectedVersion int, decidedBy uuid.UUID, comment *string) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_late_submission_requests SET
			status = 'REJECTED',
			decision_comment = $4,
			decided_by = $5,
			decided_at = now(),
			version = version + 1,
			updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3 AND status = 'REQUESTED' AND version = $6
		RETURNING `+lateSubmissionSelectColumns,
		id, tenantID, eventID, comment, decidedBy, expectedVersion,
	)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("late submission request state conflict", map[string]any{"field": "expected_version"})
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) Consume(ctx context.Context, id, tenantID, eventID, carrierCompanyID uuid.UUID, expectedVersion int, consumedAt time.Time) (*domain.LateSubmissionRequest, error) {
	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_late_submission_requests SET
			status = 'CONSUMED',
			consumed_at = $5,
			version = version + 1,
			updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND rfx_event_id = $3 AND carrier_company_id = $4
			AND status = 'APPROVED' AND version = $6
		RETURNING `+lateSubmissionSelectColumns,
		id, tenantID, eventID, carrierCompanyID, consumedAt.UTC(), expectedVersion,
	)
	req, err := scanLateSubmissionRequest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("late submission permission cannot be consumed", map[string]any{"field": "status"})
		}
		return nil, mapDBError(err)
	}
	return req, nil
}

func (r *LateSubmissionRepository) LockEventForUpdate(ctx context.Context, eventID, tenantID uuid.UUID) (*domain.RfxEvent, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, status, currency_code, valid_from, valid_to, response_deadline,
			created_at, updated_at, version
		FROM rfx.rfx_events
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`, eventID, tenantID)
	event, err := scanRfxEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx event not found")
		}
		return nil, mapDBError(err)
	}
	return event, nil
}

func scanLateSubmissionRequest(row pgx.Row) (*domain.LateSubmissionRequest, error) {
	var req domain.LateSubmissionRequest
	var reasonCode, status string
	var participantID *uuid.UUID
	var decisionComment *string
	var decidedBy *uuid.UUID
	var decidedAt *time.Time
	var consumedAt *time.Time
	var approvedFrom, approvedUntil *time.Time
	if err := row.Scan(
		&req.ID, &req.TenantID, &req.RfxEventID, &req.CarrierCompanyID, &participantID,
		&reasonCode, &req.ReasonText, &req.RequestedUntil, &status,
		&approvedFrom, &approvedUntil, &decisionComment,
		&req.RequestedBy, &decidedBy, &decidedAt, &consumedAt, &req.Version,
		&req.CreatedAt, &req.UpdatedAt,
	); err != nil {
		return nil, err
	}
	req.ParticipantID = participantID
	req.ReasonCode = domain.LateSubmissionReasonCode(reasonCode)
	req.Status = domain.LateSubmissionStatus(status)
	req.ApprovedValidFrom = approvedFrom
	req.ApprovedValidUntil = approvedUntil
	req.DecisionComment = decisionComment
	req.DecidedBy = decidedBy
	req.DecidedAt = decidedAt
	req.ConsumedAt = consumedAt
	return &req, nil
}

func scanLateSubmissionRequests(rows pgx.Rows) ([]domain.LateSubmissionRequest, error) {
	out := make([]domain.LateSubmissionRequest, 0)
	for rows.Next() {
		req, err := scanLateSubmissionRequest(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *req)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return out, nil
}
