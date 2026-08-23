//go:build integration

package analytics

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
)

type dbSettlementAccessorialReader struct {
	pool *pgxpool.Pool
}

func newDBSettlementAccessorialReader(pool *pgxpool.Pool) provider.SettlementAccessorialReader {
	return &dbSettlementAccessorialReader{pool: pool}
}

func (r *dbSettlementAccessorialReader) BatchGetSettlementsByTransportOrder(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) (map[uuid.UUID]provider.SettlementAccessorialBatchItem, error) {
	result := make(map[uuid.UUID]provider.SettlementAccessorialBatchItem)
	if len(transportOrderIDs) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (fs.transport_order_id)
			fs.transport_order_id, fs.id, fs.buyer_company_id, fs.currency_code,
			COALESCE((
				SELECT SUM(a.amount) FROM billing.settlement_accessorials a
				WHERE a.settlement_id = fs.id AND a.tenant_id = fs.tenant_id AND a.status = $3
			), 0)
		FROM billing.freight_settlements fs
		WHERE fs.tenant_id = $1
		  AND fs.transport_order_id = ANY($2)
		  AND fs.deleted_at IS NULL
		ORDER BY fs.transport_order_id, fs.created_at DESC`, tenantID, transportOrderIDs, domain.AccessorialStatusApproved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settlementByOrder := make(map[uuid.UUID]provider.SettlementAccessorialBatchItem)
	var settlementIDs []uuid.UUID
	for rows.Next() {
		var item provider.SettlementAccessorialBatchItem
		var approvedTotal decimal.Decimal
		if err := rows.Scan(
			&item.TransportOrderID, &item.SettlementID, &item.BuyerCompanyID, &item.CurrencyCode, &approvedTotal,
		); err != nil {
			return nil, err
		}
		item.ApprovedAccessorialTotal = approvedTotal.Round(domain.MoneyScale)
		settlementByOrder[item.TransportOrderID] = item
		settlementIDs = append(settlementIDs, item.SettlementID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(settlementIDs) == 0 {
		return result, nil
	}

	lineRows, err := r.pool.Query(ctx, `
		SELECT settlement_id, id, charge_code, amount, status, currency_code
		FROM billing.settlement_accessorials
		WHERE tenant_id = $1 AND settlement_id = ANY($2)
		ORDER BY created_at ASC`, tenantID, settlementIDs)
	if err != nil {
		return nil, err
	}
	defer lineRows.Close()
	linesBySettlement := make(map[uuid.UUID][]provider.SettlementAccessorialLine)
	for lineRows.Next() {
		var settlementID, accessorialID uuid.UUID
		var chargeCode, status, currency string
		var amount decimal.Decimal
		if err := lineRows.Scan(&settlementID, &accessorialID, &chargeCode, &amount, &status, &currency); err != nil {
			return nil, err
		}
		linesBySettlement[settlementID] = append(linesBySettlement[settlementID], provider.SettlementAccessorialLine{
			AccessorialID: accessorialID,
			ChargeCode:    chargeCode,
			Amount:        amount.Round(domain.MoneyScale),
			Status:        status,
			CurrencyCode:  currency,
		})
	}
	if err := lineRows.Err(); err != nil {
		return nil, err
	}
	for orderID, item := range settlementByOrder {
		item.Accessorials = linesBySettlement[item.SettlementID]
		result[orderID] = item
	}
	return result, nil
}
