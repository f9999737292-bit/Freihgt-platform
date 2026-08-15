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

type BidRevisionRepository struct {
	pool *pgxpool.Pool
	bids *BidRepository
}

func NewBidRevisionRepository(pool *pgxpool.Pool, bids *BidRepository) *BidRevisionRepository {
	return &BidRevisionRepository{pool: pool, bids: bids}
}

func (r *BidRevisionRepository) SubmitRevision(ctx context.Context, in domain.SubmitBidRevisionInput) (*domain.BidRevision, error) {
	var result *domain.BidRevision
	err := measureDB("bid_revision_repository", "submit_revision", func() error {
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

		bid, err := r.bids.GetByID(ctx, in.BidID, in.TenantID)
		if err != nil {
			return err
		}
		if bid.CarrierCompanyID != in.CarrierCompanyID {
			return apperrors.Forbidden("carrier cannot revise another carrier bid")
		}

		frDeadline, err := r.getFreightRequestDeadline(ctx, bid.FreightRequestID, in.TenantID)
		if err != nil {
			return err
		}
		if frDeadline != nil && !frDeadline.After(time.Now().UTC()) {
			return apperrors.Validation("bidding deadline has passed", map[string]any{"field": "response_deadline"})
		}

		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, `SELECT 1 FROM rfx.bids WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, in.BidID, in.TenantID); err != nil {
			return mapDBError(err)
		}

		var nextRevision int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(revision_number), 0) + 1 FROM rfx.bid_revisions WHERE bid_id = $1
		`, in.BidID).Scan(&nextRevision); err != nil {
			return mapDBError(err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE rfx.bid_revisions SET is_active = false WHERE bid_id = $1 AND is_active = true
		`, in.BidID); err != nil {
			return mapDBError(err)
		}

		now := time.Now().UTC()
		var revisionID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO rfx.bid_revisions (
				tenant_id, bid_id, revision_number, is_active,
				total_amount, currency_code, capacity_units, transit_hours,
				sla_score_input, carrier_kpi_score_input, reliability_score_input,
				comment, submitted_at, idempotency_key
			) VALUES ($1, $2, $3, true, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id, created_at
		`, in.TenantID, in.BidID, nextRevision,
			in.TotalAmount, in.CurrencyCode, in.CapacityUnits, in.TransitHours,
			in.SLAScoreInput, in.CarrierKPIInput, in.ReliabilityInput,
			optionalString(in.Comment), now, in.IdempotencyKey,
		).Scan(&revisionID, &now)
		if err != nil {
			return mapDBError(err)
		}

		_, err = tx.Exec(ctx, `
			UPDATE rfx.bids SET
				total_amount = $3, currency_code = $4, capacity_units = $5, transit_hours = $6,
				sla_score_input = $7, carrier_kpi_score_input = $8, reliability_score_input = $9,
				active_revision_number = $10, status = 'SUBMITTED', submitted_at = COALESCE(submitted_at, $11),
				updated_at = now(), version = version + 1
			WHERE id = $1 AND tenant_id = $2
		`, in.BidID, in.TenantID,
			in.TotalAmount, in.CurrencyCode, in.CapacityUnits, in.TransitHours,
			in.SLAScoreInput, in.CarrierKPIInput, in.ReliabilityInput, nextRevision, now,
		)
		if err != nil {
			return mapDBError(err)
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}

		result = &domain.BidRevision{
			ID:               revisionID,
			TenantID:         in.TenantID,
			BidID:            in.BidID,
			RevisionNumber:   nextRevision,
			IsActive:         true,
			TotalAmount:      &in.TotalAmount,
			CurrencyCode:     in.CurrencyCode,
			CapacityUnits:    &in.CapacityUnits,
			TransitHours:     &in.TransitHours,
			SLAScoreInput:    &in.SLAScoreInput,
			CarrierKPIInput:  &in.CarrierKPIInput,
			ReliabilityInput: &in.ReliabilityInput,
			Comment:          in.Comment,
			SubmittedAt:      &now,
			IdempotencyKey:   in.IdempotencyKey,
			CreatedAt:        now,
			CarrierCompanyID: bid.CarrierCompanyID,
			FreightRequestID: bid.FreightRequestID,
		}
		return nil
	})
	return result, err
}

func (r *BidRevisionRepository) getFreightRequestDeadline(ctx context.Context, frID, tenantID uuid.UUID) (*time.Time, error) {
	var deadline *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT response_deadline FROM rfx.freight_requests
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, frID, tenantID).Scan(&deadline)
	return deadline, mapDBError(err)
}

func (r *BidRevisionRepository) findByIdempotency(ctx context.Context, tenantID uuid.UUID, key string) (*domain.BidRevision, error) {
	const q = `
		SELECT rev.id, rev.tenant_id, rev.bid_id, rev.revision_number, rev.is_active,
			rev.total_amount, rev.currency_code, rev.capacity_units, rev.transit_hours,
			rev.sla_score_input, rev.carrier_kpi_score_input, rev.reliability_score_input,
			rev.comment, rev.submitted_at, rev.idempotency_key, rev.created_at,
			b.carrier_company_id, b.freight_request_id
		FROM rfx.bid_revisions rev
		JOIN rfx.bids b ON b.id = rev.bid_id
		WHERE rev.tenant_id = $1 AND rev.idempotency_key = $2
	`
	rev, err := scanBidRevision(r.pool.QueryRow(ctx, q, tenantID, key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapDBError(err)
	}
	return rev, nil
}

func (r *BidRevisionRepository) GetActiveRevision(ctx context.Context, bidID, tenantID uuid.UUID) (*domain.BidRevision, error) {
	const q = `
		SELECT rev.id, rev.tenant_id, rev.bid_id, rev.revision_number, rev.is_active,
			rev.total_amount, rev.currency_code, rev.capacity_units, rev.transit_hours,
			rev.sla_score_input, rev.carrier_kpi_score_input, rev.reliability_score_input,
			rev.comment, rev.submitted_at, rev.idempotency_key, rev.created_at,
			b.carrier_company_id, b.freight_request_id
		FROM rfx.bid_revisions rev
		JOIN rfx.bids b ON b.id = rev.bid_id
		WHERE rev.bid_id = $1 AND rev.tenant_id = $2 AND rev.is_active = true
	`
	rev, err := scanBidRevision(r.pool.QueryRow(ctx, q, bidID, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("active revision not found")
		}
		return nil, mapDBError(err)
	}
	return rev, nil
}

func (r *BidRevisionRepository) ListRevisions(ctx context.Context, bidID, tenantID uuid.UUID) ([]domain.BidRevision, error) {
	const q = `
		SELECT rev.id, rev.tenant_id, rev.bid_id, rev.revision_number, rev.is_active,
			rev.total_amount, rev.currency_code, rev.capacity_units, rev.transit_hours,
			rev.sla_score_input, rev.carrier_kpi_score_input, rev.reliability_score_input,
			rev.comment, rev.submitted_at, rev.idempotency_key, rev.created_at,
			b.carrier_company_id, b.freight_request_id
		FROM rfx.bid_revisions rev
		JOIN rfx.bids b ON b.id = rev.bid_id
		WHERE rev.bid_id = $1 AND rev.tenant_id = $2
		ORDER BY rev.revision_number ASC
	`
	rows, err := r.pool.Query(ctx, q, bidID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.BidRevision, 0)
	for rows.Next() {
		rev, err := scanBidRevision(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *rev)
	}
	return out, rows.Err()
}

func scanBidRevision(row pgx.Row) (*domain.BidRevision, error) {
	var rev domain.BidRevision
	err := row.Scan(
		&rev.ID, &rev.TenantID, &rev.BidID, &rev.RevisionNumber, &rev.IsActive,
		&rev.TotalAmount, &rev.CurrencyCode, &rev.CapacityUnits, &rev.TransitHours,
		&rev.SLAScoreInput, &rev.CarrierKPIInput, &rev.ReliabilityInput,
		&rev.Comment, &rev.SubmittedAt, &rev.IdempotencyKey, &rev.CreatedAt,
		&rev.CarrierCompanyID, &rev.FreightRequestID,
	)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}
