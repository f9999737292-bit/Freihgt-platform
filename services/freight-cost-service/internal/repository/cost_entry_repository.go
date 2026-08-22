package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type CostEntryRepository struct {
	pool *pgxpool.Pool
}

func NewCostEntryRepository(pool *pgxpool.Pool) *CostEntryRepository {
	return &CostEntryRepository{pool: pool}
}

func (r *CostEntryRepository) Insert(ctx context.Context, tx pgx.Tx, entry *domain.CostEntry) (uuid.UUID, error) {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	var amount any
	if entry.Amount != nil {
		amount = entry.Amount.StringFixed(domain.MoneyScale)
	}
	var metadata any
	if len(entry.Metadata) > 0 {
		raw, err := json.Marshal(entry.Metadata)
		if err != nil {
			return uuid.Nil, mapDBError(err)
		}
		metadata = raw
	}
	query := `
		INSERT INTO freight_cost.cost_entry (
			id, tenant_id, transport_order_id, shipment_id, buyer_company_id, carrier_company_id,
			entry_kind, amount, currency_code, tax_basis, amount_availability,
			source_service, source_type, source_id, source_revision,
			source_fact_id, source_event_id, source_occurred_at, supersedes_entry_id,
			event_origin, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20, $21
		) RETURNING id`
	args := []any{
		entry.ID, entry.TenantID, entry.TransportOrderID, optionalUUID(entry.ShipmentID),
		entry.BuyerCompanyID, entry.CarrierCompanyID,
		entry.EntryKind, amount, entry.CurrencyCode, string(entry.TaxBasis), entry.AmountAvailability,
		entry.SourceService, entry.SourceType, entry.SourceID, entry.SourceRevision,
		entry.SourceFactID, entry.SourceEventID, entry.SourceOccurredAt, optionalUUID(entry.SupersedesEntryID),
		entry.EventOrigin, metadata,
	}
	var id uuid.UUID
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, args...).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, query, args...).Scan(&id)
	}
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return id, nil
}

func (r *CostEntryRepository) FindBySourceEventID(ctx context.Context, tx pgx.Tx, tenantID, sourceEventID uuid.UUID) (*domain.CostEntry, error) {
	query := costEntrySelect + ` WHERE tenant_id = $1 AND source_event_id = $2`
	return r.scanOne(ctx, tx, query, tenantID, sourceEventID)
}

func (r *CostEntryRepository) FindBySourceFactID(ctx context.Context, tx pgx.Tx, tenantID, sourceFactID uuid.UUID) (*domain.CostEntry, error) {
	query := costEntrySelect + ` WHERE tenant_id = $1 AND source_fact_id = $2`
	return r.scanOne(ctx, tx, query, tenantID, sourceFactID)
}

func (r *CostEntryRepository) FindLatestByDimension(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
	sourceService, sourceType string,
	sourceID uuid.UUID,
	entryKind string,
) (*domain.CostEntry, error) {
	query := costEntrySelect + `
		WHERE tenant_id = $1 AND transport_order_id = $2
		  AND source_service = $3 AND source_type = $4 AND source_id = $5 AND entry_kind = $6
		ORDER BY recorded_at DESC, id DESC
		LIMIT 1`
	return r.scanOne(ctx, tx, query, tenantID, transportOrderID, sourceService, sourceType, sourceID, entryKind)
}

const costEntrySelect = `
	SELECT id, tenant_id, transport_order_id, shipment_id, buyer_company_id, carrier_company_id,
	       entry_kind, amount, currency_code, tax_basis, amount_availability,
	       source_service, source_type, source_id, source_revision,
	       source_fact_id, source_event_id, source_occurred_at, supersedes_entry_id,
	       event_origin, metadata
	FROM freight_cost.cost_entry`

func (r *CostEntryRepository) scanOne(ctx context.Context, tx pgx.Tx, query string, args ...any) (*domain.CostEntry, error) {
	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query, args...)
	} else {
		row = r.pool.QueryRow(ctx, query, args...)
	}
	entry, err := scanCostEntry(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return entry, nil
}

func scanCostEntry(row pgx.Row) (*domain.CostEntry, error) {
	var entry domain.CostEntry
	var shipmentID *uuid.UUID
	var amountStr *string
	var taxBasis string
	var supersedesID *uuid.UUID
	var metadataRaw []byte
	var occurredAt time.Time

	err := row.Scan(
		&entry.ID, &entry.TenantID, &entry.TransportOrderID, &shipmentID,
		&entry.BuyerCompanyID, &entry.CarrierCompanyID,
		&entry.EntryKind, &amountStr, &entry.CurrencyCode, &taxBasis, &entry.AmountAvailability,
		&entry.SourceService, &entry.SourceType, &entry.SourceID, &entry.SourceRevision,
		&entry.SourceFactID, &entry.SourceEventID, &occurredAt, &supersedesID,
		&entry.EventOrigin, &metadataRaw,
	)
	if err != nil {
		return nil, err
	}
	entry.ShipmentID = shipmentID
	entry.TaxBasis = domain.TaxBasis(taxBasis)
	entry.SourceOccurredAt = occurredAt.UTC()
	entry.SupersedesEntryID = supersedesID
	if amountStr != nil {
		parsed, err := domain.ParseMoneyAmount(*amountStr)
		if err != nil {
			return nil, err
		}
		entry.Amount = &parsed
	}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &entry.Metadata)
	}
	return &entry, nil
}

func optionalUUID(v *uuid.UUID) any {
	if v == nil {
		return nil
	}
	return *v
}
