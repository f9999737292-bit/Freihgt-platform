package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type BidRepository struct {
	pool *pgxpool.Pool
}

func NewBidRepository(pool *pgxpool.Pool) *BidRepository {
	return &BidRepository{pool: pool}
}

func (r *BidRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *BidRepository) CreateBid(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error) {
	var result *domain.Bid
	err := measureDB("bid_repository", "create_bid", func() error {
		totals := domain.CalculateBidTotals(in.Items, in.VATRate)

		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		const insertBid = `
		INSERT INTO rfx.bids (
			tenant_id, freight_request_id, carrier_company_id, bid_number, status,
			total_amount, currency_code, vat_rate, vat_amount, total_amount_with_vat, valid_until
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, tenant_id, freight_request_id, carrier_company_id, bid_number, status,
			total_amount, currency_code, vat_rate, vat_amount, total_amount_with_vat,
			valid_until, submitted_at, created_at, updated_at, version
	`
		row := tx.QueryRow(ctx, insertBid,
			in.TenantID,
			in.FreightRequestID,
			in.CarrierCompanyID,
			strings.TrimSpace(in.BidNumber),
			domain.BidStatusDraft,
			totals.TotalAmount,
			optionalString(in.CurrencyCode),
			optionalFloat(in.VATRate),
			totals.VATAmount,
			totals.TotalAmountWithVAT,
			in.ValidUntil,
		)
		bid, err := scanBid(row)
		if err != nil {
			return mapDBError(err)
		}

		const insertItem = `
		INSERT INTO rfx.bid_items (
			tenant_id, bid_id, description, base_amount, fuel_surcharge, toll_amount,
			extra_charges, amount_without_vat, vat_rate, vat_amount, amount_with_vat, comment
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, bid_id, description, base_amount, fuel_surcharge, toll_amount,
			extra_charges, amount_without_vat, vat_rate, vat_amount, amount_with_vat, comment
	`

		items := make([]domain.BidItem, 0, len(totals.Items))
		for _, calculated := range totals.Items {
			itemRow := tx.QueryRow(ctx, insertItem,
				in.TenantID,
				bid.ID,
				optionalString(calculated.Item.Description),
				calculated.Item.BaseAmount,
				calculated.Item.FuelSurcharge,
				calculated.Item.TollAmount,
				calculated.Item.ExtraCharges,
				calculated.AmountWithoutVAT,
				optionalFloat(calculated.Item.VATRate),
				calculated.VATAmount,
				calculated.AmountWithVAT,
				optionalString(calculated.Item.Comment),
			)
			item, err := scanBidItem(itemRow)
			if err != nil {
				return mapDBError(err)
			}
			items = append(items, *item)
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}

		bid.Items = items
		result = bid
		return nil
	})
	return result, err
}

func (r *BidRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error) {
	const bidQuery = `
		SELECT id, tenant_id, freight_request_id, carrier_company_id, bid_number, status,
			total_amount, currency_code, vat_rate, vat_amount, total_amount_with_vat,
			valid_until, submitted_at, created_at, updated_at, version
		FROM rfx.bids
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	row := r.pool.QueryRow(ctx, bidQuery, id, tenantID)
	bid, err := scanBid(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("bid not found")
		}
		return nil, mapDBError(err)
	}

	items, err := r.listBidItems(ctx, bid.ID, tenantID)
	if err != nil {
		return nil, err
	}
	bid.Items = items
	return bid, nil
}

func (r *BidRepository) ListByFreightRequest(ctx context.Context, freightRequestID, tenantID uuid.UUID) ([]domain.Bid, error) {
	var bids []domain.Bid
	err := measureDB("bid_repository", "list_bids", func() error {
		const query = `
		SELECT id, tenant_id, freight_request_id, carrier_company_id, bid_number, status,
			total_amount, currency_code, vat_rate, vat_amount, total_amount_with_vat,
			valid_until, submitted_at, created_at, updated_at, version
		FROM rfx.bids
		WHERE freight_request_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
		rows, err := r.pool.Query(ctx, query, freightRequestID, tenantID)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		bids = make([]domain.Bid, 0)
		for rows.Next() {
			bid, err := scanBid(rows)
			if err != nil {
				return mapDBError(err)
			}
			bids = append(bids, *bid)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return bids, err
}

func (r *BidRepository) SubmitBid(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.Bid, error) {
	var result *domain.Bid
	err := measureDB("bid_repository", "submit_bid", func() error {
		current, err := r.GetByID(ctx, id, tenantID)
		if err != nil {
			return err
		}

		const query = `
		UPDATE rfx.bids SET
			status = $3,
			submitted_at = now(),
			submitted_by = $4,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = $5 AND version = $6
		RETURNING id, tenant_id, freight_request_id, carrier_company_id, bid_number, status,
			total_amount, currency_code, vat_rate, vat_amount, total_amount_with_vat,
			valid_until, submitted_at, created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query,
			id,
			tenantID,
			domain.BidStatusSubmitted,
			optionalUUID(submittedBy),
			domain.BidStatusDraft,
			current.Version,
		)
		bid, err := scanBid(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("bid was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}
		bid.Items = current.Items
		result = bid
		return nil
	})
	return result, err
}

func (r *BidRepository) AcceptBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error) {
	var result *domain.Bid
	err := measureDB("bid_repository", "accept_bid", func() error {
		current, err := r.GetByID(ctx, id, tenantID)
		if err != nil {
			return err
		}

		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		const acceptBid = `
		UPDATE rfx.bids SET
			status = $3,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = $4 AND version = $5
		RETURNING id, tenant_id, freight_request_id, carrier_company_id, bid_number, status,
			total_amount, currency_code, vat_rate, vat_amount, total_amount_with_vat,
			valid_until, submitted_at, created_at, updated_at, version
	`
		row := tx.QueryRow(ctx, acceptBid,
			id,
			tenantID,
			domain.BidStatusAccepted,
			domain.BidStatusSubmitted,
			current.Version,
		)
		bid, err := scanBid(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("bid was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}

		const rejectOthers = `
		UPDATE rfx.bids SET
			status = $4,
			updated_at = now(),
			version = version + 1
		WHERE freight_request_id = $1 AND tenant_id = $2 AND id <> $3
			AND deleted_at IS NULL AND status = $5
	`
		if _, err := tx.Exec(ctx, rejectOthers,
			bid.FreightRequestID,
			tenantID,
			bid.ID,
			domain.BidStatusRejected,
			domain.BidStatusSubmitted,
		); err != nil {
			return mapDBError(err)
		}

		const updateFreightRequest = `
		UPDATE rfx.freight_requests SET
			status = $3,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
		// TODO: also update linked transport order status when transport-order integration is available.
		if _, err := tx.Exec(ctx, updateFreightRequest,
			bid.FreightRequestID,
			tenantID,
			domain.FreightRequestStatusAwarded,
		); err != nil {
			return mapDBError(err)
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}

		bid.Items = current.Items
		result = bid
		return nil
	})
	return result, err
}

func (r *BidRepository) listBidItems(ctx context.Context, bidID, tenantID uuid.UUID) ([]domain.BidItem, error) {
	const query = `
		SELECT id, bid_id, description, base_amount, fuel_surcharge, toll_amount,
			extra_charges, amount_without_vat, vat_rate, vat_amount, amount_with_vat, comment
		FROM rfx.bid_items
		WHERE bid_id = $1 AND tenant_id = $2
		ORDER BY created_at
	`
	rows, err := r.pool.Query(ctx, query, bidID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.BidItem, 0)
	for rows.Next() {
		item, err := scanBidItem(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return items, nil
}

func scanBid(row pgx.Row) (*domain.Bid, error) {
	var bid domain.Bid
	err := row.Scan(
		&bid.ID,
		&bid.TenantID,
		&bid.FreightRequestID,
		&bid.CarrierCompanyID,
		&bid.BidNumber,
		&bid.Status,
		&bid.TotalAmount,
		&bid.CurrencyCode,
		&bid.VATRate,
		&bid.VATAmount,
		&bid.TotalAmountWithVAT,
		&bid.ValidUntil,
		&bid.SubmittedAt,
		&bid.CreatedAt,
		&bid.UpdatedAt,
		&bid.Version,
	)
	if err != nil {
		return nil, err
	}
	return &bid, nil
}

func scanBidItem(row pgx.Row) (*domain.BidItem, error) {
	var item domain.BidItem
	err := row.Scan(
		&item.ID,
		&item.BidID,
		&item.Description,
		&item.BaseAmount,
		&item.FuelSurcharge,
		&item.TollAmount,
		&item.ExtraCharges,
		&item.AmountWithoutVAT,
		&item.VATRate,
		&item.VATAmount,
		&item.AmountWithVAT,
		&item.Comment,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}
