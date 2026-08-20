package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/contract-rate-service/internal/domain"
)

type ResolutionRepository struct {
	pool  *pgxpool.Pool
	audit *AuditRepository
}

func NewResolutionRepository(pool *pgxpool.Pool, audit *AuditRepository) *ResolutionRepository {
	return &ResolutionRepository{pool: pool, audit: audit}
}

func (r *ResolutionRepository) RecordManualSpotAudit(ctx context.Context, actor domain.ActorInput, correlationID *string, metadata map[string]any) (uuid.UUID, error) {
	auditID := uuid.New()
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		return r.audit.InsertTx(ctx, tx, AuditInsert{
			TenantID:       actor.TenantID,
			EntityType:     domain.AuditEntityRateLine,
			EntityID:       auditID,
			Action:         domain.AuditActionManualSpotResolved,
			ActorUserID:    &actor.ActorUserID,
			ActorCompanyID: &actor.ActorCompanyID,
			CorrelationID:  correlationID,
			Metadata:       metadata,
		})
	})
	if err != nil {
		return uuid.Nil, err
	}
	return auditID, nil
}

func (r *ResolutionRepository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapDBError(err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return mapDBError(tx.Commit(ctx))
}

type candidateRow struct {
	Candidate domain.RateCandidate
	LineID    uuid.UUID
}

func (r *ResolutionRepository) FindCandidates(ctx context.Context, req domain.ResolveRateRequest) ([]domain.RateCandidate, error) {
	const query = `
		SELECT tc.id, tc.contract_number, tc.valid_from, tc.valid_to, tc.status, tc.currency_code,
			tc.buyer_company_id, tc.carrier_company_id,
			rc.id, rc.name,
			v.id, v.version_number, v.valid_from, v.valid_to,
			l.id, l.origin_location_id, l.destination_location_id, l.equipment_type, l.transport_mode,
			c.id, c.component_type, c.calculation_method, c.amount, c.percent_value, c.unit_code
		FROM contract_rate.transport_contract tc
		JOIN contract_rate.rate_card rc ON rc.tenant_id = tc.tenant_id AND rc.contract_id = tc.id
		JOIN contract_rate.rate_card_version v ON v.tenant_id = rc.tenant_id AND v.rate_card_id = rc.id
		JOIN contract_rate.rate_line l ON l.tenant_id = v.tenant_id AND l.rate_card_version_id = v.id
		JOIN contract_rate.rate_component c ON c.tenant_id = l.tenant_id AND c.rate_line_id = l.id
		WHERE tc.tenant_id = $1
		  AND tc.buyer_company_id = $2
		  AND tc.carrier_company_id = $3
		  AND tc.status = 'ACTIVE'
		  AND v.status = 'ACTIVE'
		  AND l.origin_location_id = $4
		  AND l.destination_location_id = $5
		  AND l.equipment_type = $6
		  AND l.transport_mode = $7
		ORDER BY tc.id, rc.id, v.id, l.id, c.component_type`
	rows, err := r.pool.Query(ctx, query,
		req.TenantID, req.BuyerCompanyID, req.CarrierCompanyID,
		req.OriginLocationID, req.DestinationLocationID, req.EquipmentType, req.TransportMode,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	grouped := map[uuid.UUID]*domain.RateCandidate{}
	order := make([]uuid.UUID, 0)
	for rows.Next() {
		var contractID, buyerID, carrierID, cardID, versionID, lineID, componentID uuid.UUID
		var contractNumber, cardName, equipment, mode, componentType, calcMethod string
		var contractStatus, currency string
		var contractFrom, versionFrom time.Time
		var contractTo, versionTo *time.Time
		var versionNumber int
		var amount, percent *decimal.Decimal
		var unitCode *string
		var originID, destID uuid.UUID
		if err := rows.Scan(
			&contractID, &contractNumber, &contractFrom, &contractTo, &contractStatus, &currency,
			&buyerID, &carrierID,
			&cardID, &cardName,
			&versionID, &versionNumber, &versionFrom, &versionTo,
			&lineID, &originID, &destID, &equipment, &mode,
			&componentID, &componentType, &calcMethod, &amount, &percent, &unitCode,
		); err != nil {
			return nil, mapDBError(err)
		}
		candidate, ok := grouped[lineID]
		if !ok {
			candidate = &domain.RateCandidate{
				ContractID: contractID, ContractNumber: contractNumber,
				ContractValidFrom: contractFrom, ContractValidTo: contractTo,
				ContractStatus: contractStatus, ContractCurrency: currency,
				BuyerCompanyID: buyerID, CarrierCompanyID: carrierID,
				RateCardID: cardID, RateCardName: cardName,
				RateVersionID: versionID, VersionNumber: versionNumber,
				VersionValidFrom: versionFrom, VersionValidTo: versionTo,
				RateLineID: lineID,
				OriginLocationID: originID, DestinationLocationID: destID,
				EquipmentType: equipment, TransportMode: mode,
				Components: make([]domain.RateComponent, 0, 4),
			}
			grouped[lineID] = candidate
			order = append(order, lineID)
		}
		candidate.Components = append(candidate.Components, domain.RateComponent{
			ID: componentID, TenantID: req.TenantID, RateLineID: lineID,
			ComponentType: componentType, CalculationMethod: calcMethod,
			Amount: amount, PercentValue: percent, UnitCode: unitCode,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	result := make([]domain.RateCandidate, 0, len(order))
	for _, lineID := range order {
		result = append(result, *grouped[lineID])
	}
	return result, nil
}
