package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type ShipmentSettlementContext struct {
	ShipmentID       uuid.UUID
	TransportOrderID uuid.UUID
	ShipmentStatus   string
	BuyerCompanyID   uuid.UUID
	CarrierCompanyID uuid.UUID
	AwardLinkID      uuid.UUID
	BaseAmount       float64
	CurrencyCode     string
	VATRate          *float64
	HasPOD           bool
}

type SettlementDetail struct {
	Settlement   domain.FreightSettlement
	Accessorials []domain.SettlementAccessorial
	Disputes     []domain.SettlementDispute
}

type FreightSettlementRepository struct {
	pool *pgxpool.Pool
}

func NewFreightSettlementRepository(pool *pgxpool.Pool) *FreightSettlementRepository {
	return &FreightSettlementRepository{pool: pool}
}

func (r *FreightSettlementRepository) LoadShipmentContext(ctx context.Context, tenantID, shipmentID uuid.UUID) (*ShipmentSettlementContext, error) {
	const shipmentQuery = `
		SELECT s.id, s.transport_order_id, s.status, s.carrier_company_id
		FROM transport.shipments s
		WHERE s.id = $1 AND s.tenant_id = $2 AND s.deleted_at IS NULL`
	var ctxOut ShipmentSettlementContext
	var carrierID uuid.UUID
	err := r.pool.QueryRow(ctx, shipmentQuery, shipmentID, tenantID).Scan(
		&ctxOut.ShipmentID, &ctxOut.TransportOrderID, &ctxOut.ShipmentStatus, &carrierID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("shipment not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	ctxOut.CarrierCompanyID = carrierID

	const awardQuery = `
		SELECT id, buyer_company_id, carrier_company_id, amount::float8, currency_code, NULL::float8
		FROM rfx.rfx_award_transport_orders
		WHERE tenant_id = $1 AND transport_order_id = $2`
	var vatRate *float64
	err = r.pool.QueryRow(ctx, awardQuery, tenantID, ctxOut.TransportOrderID).Scan(
		&ctxOut.AwardLinkID, &ctxOut.BuyerCompanyID, &ctxOut.CarrierCompanyID,
		&ctxOut.BaseAmount, &ctxOut.CurrencyCode, &vatRate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.Validation("transport order has no commercial award provenance", map[string]any{"field": "shipment_id"})
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	if ctxOut.CarrierCompanyID != carrierID {
		return nil, apperrors.Validation("shipment carrier does not match award provenance", map[string]any{"field": "shipment_id"})
	}
	ctxOut.VATRate = vatRate
	ctxOut.CurrencyCode = domain.NormalizeCurrencyCode(ctxOut.CurrencyCode)

	const podQuery = `
		SELECT EXISTS (
			SELECT 1 FROM documents.documents d
			WHERE d.tenant_id = $1 AND d.related_entity_type = 'SHIPMENT'
			  AND d.related_entity_id = $2 AND d.document_type = 'POD'
			  AND d.deleted_at IS NULL
		)`
	if err := r.pool.QueryRow(ctx, podQuery, tenantID, shipmentID).Scan(&ctxOut.HasPOD); err != nil {
		return nil, mapDBError(err)
	}
	return &ctxOut, nil
}

func (r *FreightSettlementRepository) CreateSettlement(ctx context.Context, in domain.CreateFreightSettlementInput, ctxData *ShipmentSettlementContext) (*domain.FreightSettlement, error) {
	if err := domain.ValidateShipmentEligibleForSettlement(ctxData.ShipmentStatus, ctxData.HasPOD); err != nil {
		return nil, err
	}
	if err := domain.ValidateSettlementAccess(&domain.FreightSettlement{
		BuyerCompanyID: ctxData.BuyerCompanyID, CarrierCompanyID: ctxData.CarrierCompanyID,
	}, in.ActorCompanyID, in.ActorKind); err != nil {
		return nil, err
	}
	number := strings.TrimSpace(in.SettlementNumber)
	if number == "" {
		number = fmt.Sprintf("FS-%s", in.ShipmentID.String()[:8])
	}
	withoutVAT, vat, withVAT := domain.CalculateSettlementTotals(ctxData.BaseAmount, 0, ctxData.VATRate)
	var result domain.FreightSettlement
	err := r.withAuditTx(ctx, func(tx pgx.Tx) error {
		if strings.TrimSpace(in.IdempotencyKey) != "" {
			existing, findErr := findSettlementByIdempotency(ctx, tx, in.TenantID, in.IdempotencyKey)
			if findErr != nil {
				return findErr
			}
			if existing != nil {
				result = *existing
				return nil
			}
		}
		existingShipment, findErr := findSettlementByShipment(ctx, tx, in.TenantID, in.ShipmentID)
		if findErr != nil {
			return findErr
		}
		if existingShipment != nil {
			result = *existingShipment
			return nil
		}
		const insert = `
			INSERT INTO billing.freight_settlements (
				tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
				award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
				approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
				status, idempotency_key, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
			RETURNING id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
				award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
				approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
				status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
				idempotency_key, version, created_at, created_by, updated_at`
		var idempotency any
		if strings.TrimSpace(in.IdempotencyKey) != "" {
			idempotency = in.IdempotencyKey
		}
		row := tx.QueryRow(ctx, insert,
			in.TenantID, in.ShipmentID, ctxData.TransportOrderID, ctxData.BuyerCompanyID, ctxData.CarrierCompanyID,
			ctxData.AwardLinkID, number, ctxData.BaseAmount, ctxData.CurrencyCode, optionalFloat(ctxData.VATRate),
			0, withoutVAT, vat, withVAT, domain.SettlementStatusDraft, idempotency, in.ActorUserID,
		)
		created, scanErr := scanSettlement(row)
		if scanErr != nil {
			return scanErr
		}
		result = *created
		return insertAuditEvent(ctx, tx, in.TenantID, result.ID, "SETTLEMENT_CREATED", in.ActorUserID, in.ActorCompanyID, map[string]any{
			"shipment_id": in.ShipmentID.String(), "base_freight_amount": ctxData.BaseAmount,
		})
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *FreightSettlementRepository) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightSettlement, error) {
	const query = `
		SELECT id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
			award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
			approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
			status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
			idempotency_key, version, created_at, created_by, updated_at
		FROM billing.freight_settlements
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	row := r.pool.QueryRow(ctx, query, id, tenantID)
	settlement, err := scanSettlement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("freight settlement not found")
	}
	return settlement, err
}

func (r *FreightSettlementRepository) GetDetail(ctx context.Context, id, tenantID uuid.UUID) (*SettlementDetail, error) {
	settlement, err := r.GetByIDAndTenant(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	accessorials, err := r.listAccessorials(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	disputes, err := r.listDisputes(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	return &SettlementDetail{Settlement: *settlement, Accessorials: accessorials, Disputes: disputes}, nil
}

func (r *FreightSettlementRepository) List(ctx context.Context, filter domain.ListFreightSettlementsFilter) ([]domain.FreightSettlement, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	args := []any{filter.TenantID}
	where := "tenant_id = $1 AND deleted_at IS NULL"
	idx := 2
	if filter.BuyerCompanyID != nil {
		where += fmt.Sprintf(" AND buyer_company_id = $%d", idx)
		args = append(args, *filter.BuyerCompanyID)
		idx++
	}
	if filter.CarrierCompanyID != nil {
		where += fmt.Sprintf(" AND carrier_company_id = $%d", idx)
		args = append(args, *filter.CarrierCompanyID)
		idx++
	}
	if filter.Status != nil {
		where += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, *filter.Status)
		idx++
	}
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM billing.freight_settlements WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, mapDBError(err)
	}
	query := fmt.Sprintf(`
		SELECT id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
			award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
			approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
			status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
			idempotency_key, version, created_at, created_by, updated_at
		FROM billing.freight_settlements WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`, where, filter.Limit, filter.Offset)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.FreightSettlement, 0)
	for rows.Next() {
		item, scanErr := scanSettlement(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *FreightSettlementRepository) ProposeAccessorial(ctx context.Context, settlementID uuid.UUID, in domain.ProposeAccessorialInput) (*domain.SettlementAccessorial, error) {
	settlement, err := r.GetByIDAndTenant(ctx, settlementID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if in.ActorKind != domain.SettlementActorCarrier || settlement.CarrierCompanyID != in.ActorCompanyID {
		return nil, apperrors.Forbidden("only assigned carrier can propose accessorial charges")
	}
	if settlement.Status != domain.SettlementStatusDraft && settlement.Status != domain.SettlementStatusUnderReview {
		return nil, apperrors.Validation("accessorials can only be proposed in DRAFT or UNDER_REVIEW", map[string]any{"status": settlement.Status})
	}
	if in.EvidenceDocumentID != nil {
		if err := r.validateEvidenceDocument(ctx, in.TenantID, settlement.ShipmentID, *in.EvidenceDocumentID); err != nil {
			return nil, err
		}
	}
	var result domain.SettlementAccessorial
	err = r.withAuditTx(ctx, func(tx pgx.Tx) error {
		const insert = `
			INSERT INTO billing.settlement_accessorials (
				tenant_id, settlement_id, charge_code, description, amount, currency_code,
				status, submitted_by, submitted_by_company_id, evidence_document_id, evidence_type
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			RETURNING id, tenant_id, settlement_id, charge_code, description, amount, currency_code,
				status, submitted_by, submitted_by_company_id, evidence_document_id, evidence_type,
				created_at, updated_at`
		row := tx.QueryRow(ctx, insert,
			in.TenantID, settlementID, in.ChargeCode, optionalString(in.Description), in.Amount,
			settlement.CurrencyCode, domain.AccessorialStatusProposed, in.ActorUserID, in.ActorCompanyID,
			optionalUUID(in.EvidenceDocumentID), optionalString(in.EvidenceType),
		)
		item, scanErr := scanAccessorial(row)
		if scanErr != nil {
			return scanErr
		}
		result = *item
		return insertAuditEvent(ctx, tx, in.TenantID, settlementID, "ACCESSORIAL_PROPOSED", in.ActorUserID, in.ActorCompanyID, map[string]any{
			"accessorial_id": result.ID.String(), "amount": in.Amount, "charge_code": in.ChargeCode,
		})
	})
	return &result, err
}

func (r *FreightSettlementRepository) ReviewAccessorial(ctx context.Context, settlementID, accessorialID uuid.UUID, in domain.SettlementActorInput, approve bool) (*SettlementDetail, error) {
	settlement, err := r.GetByIDAndTenant(ctx, settlementID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if in.ActorKind != domain.SettlementActorBuyer || settlement.BuyerCompanyID != in.ActorCompanyID {
		return nil, apperrors.Forbidden("only buyer can approve or reject accessorial charges")
	}
	newStatus := domain.AccessorialStatusRejected
	eventType := "ACCESSORIAL_REJECTED"
	if approve {
		newStatus = domain.AccessorialStatusApproved
		eventType = "ACCESSORIAL_APPROVED"
	}
	err = r.withAuditTx(ctx, func(tx pgx.Tx) error {
		const update = `
			UPDATE billing.settlement_accessorials
			SET status = $4, updated_at = now()
			WHERE id = $1 AND settlement_id = $2 AND tenant_id = $3 AND status = $5`
		tag, execErr := tx.Exec(ctx, update, accessorialID, settlementID, in.TenantID, newStatus, domain.AccessorialStatusProposed)
		if execErr != nil {
			return mapDBError(execErr)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("accessorial not found or not in PROPOSED status")
		}
		if err := r.recalculateSettlementTotals(ctx, tx, settlement); err != nil {
			return err
		}
		return insertAuditEvent(ctx, tx, in.TenantID, settlementID, eventType, in.ActorUserID, in.ActorCompanyID, map[string]any{
			"accessorial_id": accessorialID.String(),
		})
	})
	if err != nil {
		return nil, err
	}
	return r.GetDetail(ctx, settlementID, in.TenantID)
}

func (r *FreightSettlementRepository) RaiseDispute(ctx context.Context, settlementID uuid.UUID, in domain.RaiseDisputeInput) (*domain.SettlementDispute, error) {
	settlement, err := r.GetByIDAndTenant(ctx, settlementID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSettlementAccess(settlement, in.ActorCompanyID, in.ActorKind); err != nil {
		return nil, err
	}
	var result domain.SettlementDispute
	err = r.withAuditTx(ctx, func(tx pgx.Tx) error {
		if in.AccessorialID != nil {
			const mark = `UPDATE billing.settlement_accessorials SET status = $4, updated_at = now()
				WHERE id = $1 AND settlement_id = $2 AND tenant_id = $3`
			if _, execErr := tx.Exec(ctx, mark, *in.AccessorialID, settlementID, in.TenantID, domain.AccessorialStatusDisputed); execErr != nil {
				return mapDBError(execErr)
			}
		}
		const insert = `
			INSERT INTO billing.settlement_disputes (
				tenant_id, settlement_id, accessorial_id, reason, raised_by, raised_by_company_id, status
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
			RETURNING id, tenant_id, settlement_id, accessorial_id, reason, raised_by, raised_by_company_id,
				status, resolution_note, resolved_by, resolved_at, created_at, updated_at`
		row := tx.QueryRow(ctx, insert,
			in.TenantID, settlementID, optionalUUID(in.AccessorialID), in.Reason,
			in.ActorUserID, in.ActorCompanyID, domain.DisputeStatusOpen,
		)
		dispute, scanErr := scanDispute(row)
		if scanErr != nil {
			return scanErr
		}
		result = *dispute
		if settlement.Status != domain.SettlementStatusDisputed {
			if err := updateSettlementStatus(ctx, tx, settlement, domain.SettlementStatusDisputed); err != nil {
				return err
			}
		}
		return insertAuditEvent(ctx, tx, in.TenantID, settlementID, "DISPUTE_RAISED", in.ActorUserID, in.ActorCompanyID, map[string]any{
			"dispute_id": result.ID.String(),
		})
	})
	return &result, err
}

func (r *FreightSettlementRepository) ResolveDispute(ctx context.Context, settlementID, disputeID uuid.UUID, in domain.ResolveDisputeInput) (*SettlementDetail, error) {
	settlement, err := r.GetByIDAndTenant(ctx, settlementID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if in.ActorKind != domain.SettlementActorBuyer || settlement.BuyerCompanyID != in.ActorCompanyID {
		return nil, apperrors.Forbidden("only buyer can resolve disputes")
	}
	err = r.withAuditTx(ctx, func(tx pgx.Tx) error {
		const update = `
			UPDATE billing.settlement_disputes
			SET status = $4, resolution_note = $5, resolved_by = $6, resolved_at = now(), updated_at = now()
			WHERE id = $1 AND settlement_id = $2 AND tenant_id = $3 AND status = $7`
		tag, execErr := tx.Exec(ctx, update, disputeID, settlementID, in.TenantID,
			domain.DisputeStatusResolved, strings.TrimSpace(in.ResolutionNote), in.ActorUserID, domain.DisputeStatusOpen)
		if execErr != nil {
			return mapDBError(execErr)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("open dispute not found")
		}
		openCount, countErr := countOpenDisputes(ctx, tx, settlementID, in.TenantID)
		if countErr != nil {
			return countErr
		}
		if openCount == 0 && settlement.Status == domain.SettlementStatusDisputed {
			if err := updateSettlementStatus(ctx, tx, settlement, domain.SettlementStatusUnderReview); err != nil {
				return err
			}
		}
		return insertAuditEvent(ctx, tx, in.TenantID, settlementID, "DISPUTE_RESOLVED", in.ActorUserID, in.ActorCompanyID, map[string]any{
			"dispute_id": disputeID.String(),
		})
	})
	if err != nil {
		return nil, err
	}
	return r.GetDetail(ctx, settlementID, in.TenantID)
}

func (r *FreightSettlementRepository) TransitionStatus(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput, toStatus string) (*domain.FreightSettlement, error) {
	settlement, err := r.GetByIDAndTenant(ctx, settlementID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateSettlementAccess(settlement, in.ActorCompanyID, in.ActorKind); err != nil {
		return nil, err
	}
	if err := domain.ValidateSettlementTransition(settlement.Status, toStatus); err != nil {
		return nil, err
	}
	switch toStatus {
	case domain.SettlementStatusUnderReview:
		if in.ActorKind != domain.SettlementActorCarrier {
			return nil, apperrors.Forbidden("only carrier can submit settlement for review")
		}
	case domain.SettlementStatusApproved:
		if in.ActorKind != domain.SettlementActorBuyer {
			return nil, apperrors.Forbidden("only buyer can approve settlement")
		}
		if open, openErr := r.hasOpenDisputes(ctx, settlementID, in.TenantID); openErr != nil {
			return nil, openErr
		} else if open {
			return nil, apperrors.Validation("open disputes must be resolved before approval", map[string]any{"field": "status"})
		}
	case domain.SettlementStatusDocumentsReady, domain.SettlementStatusReadyForPayment:
		if in.ActorKind != domain.SettlementActorBuyer {
			return nil, apperrors.Forbidden("only buyer can advance document/payment readiness")
		}
	}
	var result domain.FreightSettlement
	err = r.withAuditTx(ctx, func(tx pgx.Tx) error {
		if toStatus == domain.SettlementStatusApproved {
			const accept = `
				UPDATE billing.freight_settlements
				SET status = $3, service_accepted_at = now(), service_accepted_by = $4, updated_at = now(), version = version + 1
				WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
				RETURNING id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
					award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
					approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
					status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
					idempotency_key, version, created_at, created_by, updated_at`
			row := tx.QueryRow(ctx, accept, settlementID, in.TenantID, toStatus, in.ActorUserID)
			updated, scanErr := scanSettlement(row)
			if scanErr != nil {
				return scanErr
			}
			result = *updated
		} else {
			if err := updateSettlementStatus(ctx, tx, settlement, toStatus); err != nil {
				return err
			}
			updated, getErr := r.getByIDTx(ctx, tx, settlementID, in.TenantID)
			if getErr != nil {
				return getErr
			}
			result = *updated
		}
		return insertAuditEvent(ctx, tx, in.TenantID, settlementID, "SETTLEMENT_STATUS_"+toStatus, in.ActorUserID, in.ActorCompanyID, map[string]any{
			"from": settlement.Status, "to": toStatus,
		})
	})
	return &result, err
}

func (r *FreightSettlementRepository) IncludeInRegister(ctx context.Context, settlementID uuid.UUID, in domain.SettlementActorInput, registerNumber string) (*domain.FreightSettlement, error) {
	settlement, err := r.GetByIDAndTenant(ctx, settlementID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if settlement.BillingRegisterID != nil {
		return settlement, nil
	}
	if settlement.Status != domain.SettlementStatusApproved &&
		settlement.Status != domain.SettlementStatusDocumentsReady &&
		settlement.Status != domain.SettlementStatusReadyForPayment {
		return nil, apperrors.Validation("settlement must be approved before register inclusion", map[string]any{"status": settlement.Status})
	}
	if in.ActorKind != domain.SettlementActorBuyer || settlement.BuyerCompanyID != in.ActorCompanyID {
		return nil, apperrors.Forbidden("only buyer can include settlement in billing register")
	}
	var result domain.FreightSettlement
	err = r.withAuditTx(ctx, func(tx pgx.Tx) error {
		regID, itemID, includeErr := includeSettlementInRegisterTx(ctx, tx, settlement, registerNumber, in.ActorUserID)
		if includeErr != nil {
			return includeErr
		}
		const update = `
			UPDATE billing.freight_settlements
			SET billing_register_id = $3, billing_register_item_id = $4, updated_at = now(), version = version + 1
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			RETURNING id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
				award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
				approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
				status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
				idempotency_key, version, created_at, created_by, updated_at`
		row := tx.QueryRow(ctx, update, settlementID, in.TenantID, regID, itemID)
		updated, scanErr := scanSettlement(row)
		if scanErr != nil {
			return scanErr
		}
		result = *updated
		return insertAuditEvent(ctx, tx, in.TenantID, settlementID, "REGISTER_INCLUDED", in.ActorUserID, in.ActorCompanyID, map[string]any{
			"billing_register_id": regID.String(), "billing_register_item_id": itemID.String(),
		})
	})
	return &result, err
}

func (r *FreightSettlementRepository) withAuditTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapDBError(err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *FreightSettlementRepository) validateEvidenceDocument(ctx context.Context, tenantID, shipmentID, documentID uuid.UUID) error {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM documents.documents
			WHERE id = $1 AND tenant_id = $2 AND related_entity_type = 'SHIPMENT'
			  AND related_entity_id = $3 AND deleted_at IS NULL
		)`
	var ok bool
	if err := r.pool.QueryRow(ctx, query, documentID, tenantID, shipmentID).Scan(&ok); err != nil {
		return mapDBError(err)
	}
	if !ok {
		return apperrors.Forbidden("evidence document is not linked to this shipment")
	}
	return nil
}

func (r *FreightSettlementRepository) recalculateSettlementTotals(ctx context.Context, tx pgx.Tx, settlement *domain.FreightSettlement) error {
	const sumQuery = `
		SELECT COALESCE(SUM(amount), 0)::float8
		FROM billing.settlement_accessorials
		WHERE settlement_id = $1 AND tenant_id = $2 AND status = $3`
	var approvedTotal float64
	if err := tx.QueryRow(ctx, sumQuery, settlement.ID, settlement.TenantID, domain.AccessorialStatusApproved).Scan(&approvedTotal); err != nil {
		return mapDBError(err)
	}
	withoutVAT, vat, withVAT := domain.CalculateSettlementTotals(settlement.BaseFreightAmount, approvedTotal, settlement.VATRate)
	const update = `
		UPDATE billing.freight_settlements
		SET approved_accessorial_total = $3, total_without_vat = $4, vat_amount = $5, total_with_vat = $6,
			updated_at = now(), version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	_, err := tx.Exec(ctx, update, settlement.ID, settlement.TenantID, approvedTotal, withoutVAT, vat, withVAT)
	return mapDBError(err)
}

func (r *FreightSettlementRepository) hasOpenDisputes(ctx context.Context, settlementID, tenantID uuid.UUID) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM billing.settlement_disputes
		WHERE settlement_id = $1 AND tenant_id = $2 AND status = $3)`
	var ok bool
	err := r.pool.QueryRow(ctx, query, settlementID, tenantID, domain.DisputeStatusOpen).Scan(&ok)
	return ok, mapDBError(err)
}

func (r *FreightSettlementRepository) listAccessorials(ctx context.Context, settlementID, tenantID uuid.UUID) ([]domain.SettlementAccessorial, error) {
	const query = `
		SELECT id, tenant_id, settlement_id, charge_code, description, amount, currency_code,
			status, submitted_by, submitted_by_company_id, evidence_document_id, evidence_type,
			created_at, updated_at
		FROM billing.settlement_accessorials
		WHERE settlement_id = $1 AND tenant_id = $2 ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, query, settlementID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.SettlementAccessorial, 0)
	for rows.Next() {
		item, scanErr := scanAccessorial(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *FreightSettlementRepository) listDisputes(ctx context.Context, settlementID, tenantID uuid.UUID) ([]domain.SettlementDispute, error) {
	const query = `
		SELECT id, tenant_id, settlement_id, accessorial_id, reason, raised_by, raised_by_company_id,
			status, resolution_note, resolved_by, resolved_at, created_at, updated_at
		FROM billing.settlement_disputes
		WHERE settlement_id = $1 AND tenant_id = $2 ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, query, settlementID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	items := make([]domain.SettlementDispute, 0)
	for rows.Next() {
		item, scanErr := scanDispute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func includeSettlementInRegisterTx(ctx context.Context, tx pgx.Tx, settlement *domain.FreightSettlement, registerNumber string, actor uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	number := strings.TrimSpace(registerNumber)
	if number == "" {
		number = fmt.Sprintf("BR-%s", settlement.SettlementNumber)
	}
	period := time.Now().UTC()
	const insertReg = `
		INSERT INTO billing.billing_registers (
			tenant_id, register_number, customer_company_id, contractor_company_id,
			period_from, period_to, currency_code, vat_rate, status, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (tenant_id, register_number) DO UPDATE SET updated_at = now()
		RETURNING id`
	var regID uuid.UUID
	if err := tx.QueryRow(ctx, insertReg,
		settlement.TenantID, number, settlement.BuyerCompanyID, settlement.CarrierCompanyID,
		period, period, settlement.CurrencyCode, optionalFloat(settlement.VATRate),
		domain.RegisterStatusDraft, actor,
	).Scan(&regID); err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	const existingItem = `SELECT id FROM billing.billing_register_items WHERE register_id = $1 AND shipment_id = $2`
	var existingItemID uuid.UUID
	err := tx.QueryRow(ctx, existingItem, regID, settlement.ShipmentID).Scan(&existingItemID)
	if err == nil {
		return regID, existingItemID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	amounts := domain.CalculateItemAmounts(settlement.BaseFreightAmount, settlement.ApprovedAccessorialTotal, 0, settlement.VATRate)
	const insertItem = `
		INSERT INTO billing.billing_register_items (
			tenant_id, register_id, settlement_id, shipment_id, transport_order_id, carrier_company_id,
			base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id`
	var itemID uuid.UUID
	if err := tx.QueryRow(ctx, insertItem,
		settlement.TenantID, regID, settlement.ID, settlement.ShipmentID, settlement.TransportOrderID, settlement.CarrierCompanyID,
		settlement.BaseFreightAmount, settlement.ApprovedAccessorialTotal, 0,
		amounts.AmountWithoutVAT, optionalFloat(settlement.VATRate), amounts.VATAmount, amounts.AmountWithVAT,
		domain.RegisterItemStatusDraft, actor,
	).Scan(&itemID); err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	const recalc = `
		UPDATE billing.billing_registers
		SET total_without_vat = (
				SELECT COALESCE(SUM(amount_without_vat),0) FROM billing.billing_register_items WHERE register_id = $1
			),
			vat_amount = (
				SELECT COALESCE(SUM(vat_amount),0) FROM billing.billing_register_items WHERE register_id = $1
			),
			total_with_vat = (
				SELECT COALESCE(SUM(amount_with_vat),0) FROM billing.billing_register_items WHERE register_id = $1
			),
			updated_at = now(), version = version + 1
		WHERE id = $1 AND tenant_id = $2`
	if _, err := tx.Exec(ctx, recalc, regID, settlement.TenantID); err != nil {
		return uuid.Nil, uuid.Nil, mapDBError(err)
	}
	return regID, itemID, nil
}

func findSettlementByIdempotency(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, key string) (*domain.FreightSettlement, error) {
	const query = `
		SELECT id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
			award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
			approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
			status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
			idempotency_key, version, created_at, created_by, updated_at
		FROM billing.freight_settlements
		WHERE tenant_id = $1 AND idempotency_key = $2 AND deleted_at IS NULL`
	row := tx.QueryRow(ctx, query, tenantID, key)
	settlement, err := scanSettlement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return settlement, err
}

func findSettlementByShipment(ctx context.Context, tx pgx.Tx, tenantID, shipmentID uuid.UUID) (*domain.FreightSettlement, error) {
	const query = `
		SELECT id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
			award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
			approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
			status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
			idempotency_key, version, created_at, created_by, updated_at
		FROM billing.freight_settlements
		WHERE tenant_id = $1 AND shipment_id = $2 AND deleted_at IS NULL`
	row := tx.QueryRow(ctx, query, tenantID, shipmentID)
	settlement, err := scanSettlement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return settlement, err
}

func updateSettlementStatus(ctx context.Context, tx pgx.Tx, settlement *domain.FreightSettlement, toStatus string) error {
	const update = `
		UPDATE billing.freight_settlements
		SET status = $3, updated_at = now(), version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	tag, err := tx.Exec(ctx, update, settlement.ID, settlement.TenantID, toStatus)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("freight settlement not found")
	}
	return nil
}

func (r *FreightSettlementRepository) getByIDTx(ctx context.Context, tx pgx.Tx, id, tenantID uuid.UUID) (*domain.FreightSettlement, error) {
	const query = `
		SELECT id, tenant_id, shipment_id, transport_order_id, buyer_company_id, carrier_company_id,
			award_link_id, settlement_number, base_freight_amount, currency_code, vat_rate,
			approved_accessorial_total, total_without_vat, vat_amount, total_with_vat,
			status, service_accepted_at, service_accepted_by, billing_register_id, billing_register_item_id,
			idempotency_key, version, created_at, created_by, updated_at
		FROM billing.freight_settlements WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	return scanSettlement(tx.QueryRow(ctx, query, id, tenantID))
}

func countOpenDisputes(ctx context.Context, tx pgx.Tx, settlementID, tenantID uuid.UUID) (int, error) {
	const query = `SELECT COUNT(*) FROM billing.settlement_disputes WHERE settlement_id = $1 AND tenant_id = $2 AND status = $3`
	var count int
	err := tx.QueryRow(ctx, query, settlementID, tenantID, domain.DisputeStatusOpen).Scan(&count)
	return count, mapDBError(err)
}

func insertAuditEvent(ctx context.Context, tx pgx.Tx, tenantID, settlementID uuid.UUID, eventType string, actorUser, actorCompany uuid.UUID, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return apperrors.Internal("marshal audit payload", err)
	}
	const insert = `
		INSERT INTO billing.settlement_audit_events (tenant_id, settlement_id, event_type, actor_user_id, actor_company_id, payload)
		VALUES ($1,$2,$3,$4,$5,$6)`
	_, err = tx.Exec(ctx, insert, tenantID, settlementID, eventType, actorUser, actorCompany, raw)
	return mapDBError(err)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanSettlement(row scannable) (*domain.FreightSettlement, error) {
	var s domain.FreightSettlement
	var awardLink, billingReg, billingItem, createdBy *uuid.UUID
	var serviceAcceptedBy *uuid.UUID
	var idempotency *string
	var serviceAcceptedAt *time.Time
	err := row.Scan(
		&s.ID, &s.TenantID, &s.ShipmentID, &s.TransportOrderID, &s.BuyerCompanyID, &s.CarrierCompanyID,
		&awardLink, &s.SettlementNumber, &s.BaseFreightAmount, &s.CurrencyCode, &s.VATRate,
		&s.ApprovedAccessorialTotal, &s.TotalWithoutVAT, &s.VATAmount, &s.TotalWithVAT,
		&s.Status, &serviceAcceptedAt, &serviceAcceptedBy, &billingReg, &billingItem,
		&idempotency, &s.Version, &s.CreatedAt, &createdBy, &s.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	s.AwardLinkID = awardLink
	s.ServiceAcceptedAt = serviceAcceptedAt
	s.ServiceAcceptedBy = serviceAcceptedBy
	s.BillingRegisterID = billingReg
	s.BillingRegisterItemID = billingItem
	s.IdempotencyKey = idempotency
	s.CreatedBy = createdBy
	return &s, nil
}

func scanAccessorial(row scannable) (*domain.SettlementAccessorial, error) {
	var a domain.SettlementAccessorial
	var description, evidenceType *string
	var evidenceID *uuid.UUID
	err := row.Scan(
		&a.ID, &a.TenantID, &a.SettlementID, &a.ChargeCode, &description, &a.Amount, &a.CurrencyCode,
		&a.Status, &a.SubmittedBy, &a.SubmittedByCompanyID, &evidenceID, &evidenceType,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	a.Description = description
	a.EvidenceDocumentID = evidenceID
	a.EvidenceType = evidenceType
	return &a, nil
}

func scanDispute(row scannable) (*domain.SettlementDispute, error) {
	var d domain.SettlementDispute
	var accessorialID, resolvedBy *uuid.UUID
	var resolutionNote *string
	var resolvedAt *time.Time
	err := row.Scan(
		&d.ID, &d.TenantID, &d.SettlementID, &accessorialID, &d.Reason, &d.RaisedBy, &d.RaisedByCompanyID,
		&d.Status, &resolutionNote, &resolvedBy, &resolvedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	d.AccessorialID = accessorialID
	d.ResolutionNote = resolutionNote
	d.ResolvedBy = resolvedBy
	d.ResolvedAt = resolvedAt
	return &d, nil
}

// SimulateAuditFailureForTest forces rollback when audit insert fails.
func (r *FreightSettlementRepository) SimulateAuditFailureForTest(ctx context.Context, settlementID, tenantID uuid.UUID) error {
	return r.withAuditTx(ctx, func(tx pgx.Tx) error {
		settlement, err := r.getByIDTx(ctx, tx, settlementID, tenantID)
		if err != nil {
			return err
		}
		if err := updateSettlementStatus(ctx, tx, settlement, domain.SettlementStatusUnderReview); err != nil {
			return err
		}
		return fmt.Errorf("simulated audit failure")
	})
}
