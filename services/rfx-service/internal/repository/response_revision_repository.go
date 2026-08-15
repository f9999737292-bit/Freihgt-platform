package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ResponseRevisionRepository struct {
	pool *pgxpool.Pool
}

func NewResponseRevisionRepository(pool *pgxpool.Pool) *ResponseRevisionRepository {
	return &ResponseRevisionRepository{pool: pool}
}

func (r *ResponseRevisionRepository) GetBiddingContext(ctx context.Context, eventID, tenantID uuid.UUID) (domain.BiddingContext, string, error) {
	const q = `
		SELECT status, rfx_type, response_deadline, bidding_closed_at
		FROM rfx.rfx_events
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	var bidding domain.BiddingContext
	var rfxType string
	if err := r.pool.QueryRow(ctx, q, eventID, tenantID).Scan(
		&bidding.Status, &rfxType, &bidding.ResponseDeadline, &bidding.BiddingClosedAt,
	); err != nil {
		return domain.BiddingContext{}, "", mapDBError(err)
	}
	return bidding, rfxType, nil
}

func (r *ResponseRevisionRepository) SubmitRevision(ctx context.Context, in domain.SubmitResponseRevisionInput) (*domain.RfxResponseRevision, error) {
	var result *domain.RfxResponseRevision
	err := measureDB("response_revision_repository", "submit_revision", func() error {
		if in.IdempotencyKey != nil && *in.IdempotencyKey != "" {
			existing, err := r.findByIdempotency(ctx, in.TenantID, *in.IdempotencyKey)
			if err != nil {
				return err
			}
			if existing != nil {
				result = existing
				return nil
			}
		}

		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		var responseEventID uuid.UUID
		var participantID uuid.UUID
		var responseStatus string
		err = tx.QueryRow(ctx, `
			SELECT rfx_event_id, participant_company_id, status
			FROM rfx.rfx_responses
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			FOR UPDATE
		`, in.RfxResponseID, in.TenantID).Scan(&responseEventID, &participantID, &responseStatus)
		if err != nil {
			return mapDBError(err)
		}
		if responseEventID != in.RfxEventID {
			return apperrors.NotFound("rfx response not found")
		}
		if participantID != in.ParticipantCompanyID {
			return apperrors.Forbidden("carrier cannot revise another participant response")
		}

		var bidCtx domain.BiddingContext
		var rfxType string
		err = tx.QueryRow(ctx, `
			SELECT status, rfx_type, response_deadline, bidding_closed_at
			FROM rfx.rfx_events WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, in.RfxEventID, in.TenantID).Scan(&bidCtx.Status, &rfxType, &bidCtx.ResponseDeadline, &bidCtx.BiddingClosedAt)
		if err != nil {
			return mapDBError(err)
		}
		if err := domain.ValidateBiddingOpen(bidCtx); err != nil {
			return err
		}

		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM rfx.rfx_participants
				WHERE rfx_event_id = $1 AND company_id = $2 AND tenant_id = $3
			)
		`, in.RfxEventID, in.ParticipantCompanyID, in.TenantID).Scan(&exists); err != nil {
			return mapDBError(err)
		}
		if !exists {
			return apperrors.NotFound("participant not found")
		}

		var nextRevision int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(revision_number), 0) + 1
			FROM rfx.rfx_response_revisions WHERE rfx_response_id = $1
		`, in.RfxResponseID).Scan(&nextRevision); err != nil {
			return mapDBError(err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE rfx.rfx_response_revisions SET is_active = false
			WHERE rfx_response_id = $1 AND is_active = true
		`, in.RfxResponseID); err != nil {
			return mapDBError(err)
		}

		now := time.Now().UTC()
		var revisionID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO rfx.rfx_response_revisions (
				tenant_id, rfx_response_id, revision_number, is_active,
				price_amount, currency_code, capacity_units, transit_hours,
				sla_score_input, carrier_kpi_score_input, reliability_score_input,
				comment, submitted_at, idempotency_key
			) VALUES ($1, $2, $3, true, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id, created_at
		`, in.TenantID, in.RfxResponseID, nextRevision,
			in.PriceAmount, in.CurrencyCode, in.CapacityUnits, in.TransitHours,
			in.SLAScoreInput, in.CarrierKPIInput, in.ReliabilityInput,
			optionalString(in.Comment), now, in.IdempotencyKey,
		).Scan(&revisionID, &now)
		if err != nil {
			return mapDBError(err)
		}

		_, err = tx.Exec(ctx, `
			UPDATE rfx.rfx_responses SET
				price_amount = $3,
				currency_code = $4,
				capacity_units = $5,
				transit_hours = $6,
				sla_score_input = $7,
				carrier_kpi_score_input = $8,
				reliability_score_input = $9,
				active_revision_number = $10,
				status = 'SUBMITTED',
				submitted_at = COALESCE(submitted_at, $11),
				updated_at = now(),
				version = version + 1
			WHERE id = $1 AND tenant_id = $2
		`, in.RfxResponseID, in.TenantID,
			in.PriceAmount, in.CurrencyCode, in.CapacityUnits, in.TransitHours,
			in.SLAScoreInput, in.CarrierKPIInput, in.ReliabilityInput,
			nextRevision, now,
		)
		if err != nil {
			return mapDBError(err)
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}

		result = &domain.RfxResponseRevision{
			ID:                   revisionID,
			TenantID:             in.TenantID,
			RfxResponseID:        in.RfxResponseID,
			RevisionNumber:       nextRevision,
			IsActive:             true,
			PriceAmount:          &in.PriceAmount,
			CurrencyCode:         in.CurrencyCode,
			CapacityUnits:        &in.CapacityUnits,
			TransitHours:         &in.TransitHours,
			SLAScoreInput:        &in.SLAScoreInput,
			CarrierKPIInput:      &in.CarrierKPIInput,
			ReliabilityInput:     &in.ReliabilityInput,
			Comment:              in.Comment,
			SubmittedAt:          &now,
			IdempotencyKey:       in.IdempotencyKey,
			CreatedAt:            now,
			ParticipantCompanyID: participantID,
			RfxEventID:           in.RfxEventID,
		}
		return nil
	})
	return result, err
}

func (r *ResponseRevisionRepository) findByIdempotency(ctx context.Context, tenantID uuid.UUID, key string) (*domain.RfxResponseRevision, error) {
	const q = `
		SELECT rev.id, rev.tenant_id, rev.rfx_response_id, rev.revision_number, rev.is_active,
			rev.price_amount, rev.currency_code, rev.capacity_units, rev.transit_hours,
			rev.sla_score_input, rev.carrier_kpi_score_input, rev.reliability_score_input,
			rev.comment, rev.submitted_at, rev.idempotency_key, rev.created_at,
			resp.participant_company_id, resp.rfx_event_id
		FROM rfx.rfx_response_revisions rev
		JOIN rfx.rfx_responses resp ON resp.id = rev.rfx_response_id
		WHERE rev.tenant_id = $1 AND rev.idempotency_key = $2
	`
	row := r.pool.QueryRow(ctx, q, tenantID, key)
	rev, err := scanResponseRevision(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	return rev, nil
}

func (r *ResponseRevisionRepository) GetActiveRevision(ctx context.Context, responseID, tenantID uuid.UUID) (*domain.RfxResponseRevision, error) {
	const q = `
		SELECT rev.id, rev.tenant_id, rev.rfx_response_id, rev.revision_number, rev.is_active,
			rev.price_amount, rev.currency_code, rev.capacity_units, rev.transit_hours,
			rev.sla_score_input, rev.carrier_kpi_score_input, rev.reliability_score_input,
			rev.comment, rev.submitted_at, rev.idempotency_key, rev.created_at,
			resp.participant_company_id, resp.rfx_event_id
		FROM rfx.rfx_response_revisions rev
		JOIN rfx.rfx_responses resp ON resp.id = rev.rfx_response_id
		WHERE rev.rfx_response_id = $1 AND rev.tenant_id = $2 AND rev.is_active = true
	`
	rev, err := scanResponseRevision(r.pool.QueryRow(ctx, q, responseID, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("active revision not found")
		}
		return nil, mapDBError(err)
	}
	return rev, nil
}

func (r *ResponseRevisionRepository) ListRevisions(ctx context.Context, responseID, tenantID uuid.UUID) ([]domain.RfxResponseRevision, error) {
	const q = `
		SELECT rev.id, rev.tenant_id, rev.rfx_response_id, rev.revision_number, rev.is_active,
			rev.price_amount, rev.currency_code, rev.capacity_units, rev.transit_hours,
			rev.sla_score_input, rev.carrier_kpi_score_input, rev.reliability_score_input,
			rev.comment, rev.submitted_at, rev.idempotency_key, rev.created_at,
			resp.participant_company_id, resp.rfx_event_id
		FROM rfx.rfx_response_revisions rev
		JOIN rfx.rfx_responses resp ON resp.id = rev.rfx_response_id
		WHERE rev.rfx_response_id = $1 AND rev.tenant_id = $2
		ORDER BY rev.revision_number ASC
	`
	rows, err := r.pool.Query(ctx, q, responseID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.RfxResponseRevision, 0)
	for rows.Next() {
		rev, err := scanResponseRevision(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *rev)
	}
	return out, rows.Err()
}

func (r *ResponseRevisionRepository) ListEventBids(ctx context.Context, eventID, tenantID uuid.UUID, carrierScope *uuid.UUID) ([]domain.RfxResponseRevision, error) {
	q := `
		SELECT rev.id, rev.tenant_id, rev.rfx_response_id, rev.revision_number, rev.is_active,
			rev.price_amount, rev.currency_code, rev.capacity_units, rev.transit_hours,
			rev.sla_score_input, rev.carrier_kpi_score_input, rev.reliability_score_input,
			rev.comment, rev.submitted_at, rev.idempotency_key, rev.created_at,
			resp.participant_company_id, resp.rfx_event_id
		FROM rfx.rfx_response_revisions rev
		JOIN rfx.rfx_responses resp ON resp.id = rev.rfx_response_id
		WHERE resp.rfx_event_id = $1 AND resp.tenant_id = $2 AND rev.is_active = true
			AND resp.deleted_at IS NULL AND resp.status = 'SUBMITTED'
	`
	args := []any{eventID, tenantID}
	if carrierScope != nil {
		q += ` AND resp.participant_company_id = $3`
		args = append(args, *carrierScope)
	}
	q += ` ORDER BY resp.participant_company_id`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.RfxResponseRevision, 0)
	for rows.Next() {
		rev, err := scanResponseRevision(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *rev)
	}
	return out, rows.Err()
}

func (r *ResponseRevisionRepository) GetResponseOwner(ctx context.Context, responseID, tenantID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var eventID, carrierID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT rfx_event_id, participant_company_id
		FROM rfx.rfx_responses WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, responseID, tenantID).Scan(&eventID, &carrierID)
	return eventID, carrierID, mapDBError(err)
}

func scanResponseRevision(row pgx.Row) (*domain.RfxResponseRevision, error) {
	var rev domain.RfxResponseRevision
	err := row.Scan(
		&rev.ID, &rev.TenantID, &rev.RfxResponseID, &rev.RevisionNumber, &rev.IsActive,
		&rev.PriceAmount, &rev.CurrencyCode, &rev.CapacityUnits, &rev.TransitHours,
		&rev.SLAScoreInput, &rev.CarrierKPIInput, &rev.ReliabilityInput,
		&rev.Comment, &rev.SubmittedAt, &rev.IdempotencyKey, &rev.CreatedAt,
		&rev.ParticipantCompanyID, &rev.RfxEventID,
	)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}
