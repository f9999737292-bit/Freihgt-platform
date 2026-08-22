package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type IncludeSettlementResult struct {
	Register *domain.BillingRegister
	Item     *domain.BillingRegisterItem
}

func (r *BillingRegisterRepository) IncludeSettlement(
	ctx context.Context,
	registerID, settlementID uuid.UUID,
	actor domain.SettlementActorInput,
) (*IncludeSettlementResult, error) {
	var result IncludeSettlementResult
	err := r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateBillingRegisterAccess(reg, actor.ActorCompanyID, actor.ActorKind); err != nil {
			return err
		}
		if actor.ActorKind != domain.SettlementActorBuyer {
			return apperrors.Forbidden("only buyer can include settlement in billing register")
		}
		if err := domain.ValidateAddItemRegisterStatus(reg.Status); err != nil {
			return err
		}

		settlement, err := getSettlementByIDTx(ctx, tx, settlementID, actor.TenantID)
		if err != nil {
			return err
		}
		if settlement.BillingRegisterID != nil {
			if *settlement.BillingRegisterID == registerID {
				item, findErr := findRegisterItemBySettlementTx(ctx, tx, registerID, settlementID, actor.TenantID)
				if findErr != nil {
					return findErr
				}
				result.Register = reg
				result.Item = item
				return nil
			}
			return apperrors.Conflict("settlement is already included in another billing register", nil)
		}

		openDisputes, err := countOpenSettlementDisputesTx(ctx, tx, settlementID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateSettlementForBillingInclusion(settlement, openDisputes > 0, reg); err != nil {
			return err
		}

		existingItem, err := findRegisterItemBySettlementAnyTx(ctx, tx, settlementID, actor.TenantID)
		if err != nil {
			return err
		}
		if existingItem != nil {
			if existingItem.RegisterID != registerID {
				return apperrors.Conflict("settlement is already billed in another register", nil)
			}
			result.Register = reg
			result.Item = existingItem
			return linkSettlementToRegisterTx(ctx, tx, r.outbox, settlement, registerID, existingItem.ID, existingItem.AmountWithoutVAT)
		}

		vatRate := settlement.VATRate
		if vatRate == nil {
			vatRate = reg.VATRate
		}
		amounts := domain.CalculateItemAmounts(settlement.BaseFreightAmount, settlement.ApprovedAccessorialTotal, 0, vatRate)
		const insert = `
			INSERT INTO billing.billing_register_items (
				tenant_id, register_id, settlement_id, shipment_id, transport_order_id, carrier_company_id,
				base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			RETURNING id, tenant_id, register_id, settlement_id, shipment_id, transport_order_id, route_description,
				pickup_date, delivery_date, shipper_company_id, consignee_company_id, carrier_company_id,
				base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, created_at`
		row := tx.QueryRow(ctx, insert,
			actor.TenantID, registerID, settlementID, settlement.ShipmentID, settlement.TransportOrderID, settlement.CarrierCompanyID,
			settlement.BaseFreightAmount, settlement.ApprovedAccessorialTotal, 0,
			amounts.AmountWithoutVAT, optionalFloat(vatRate), amounts.VATAmount, amounts.AmountWithVAT,
			domain.RegisterItemStatusDraft, actor.ActorUserID,
		)
		item, scanErr := scanItemWithSettlement(row)
		if scanErr != nil {
			return scanErr
		}
		if err := recalculateRegisterTotalsTx(ctx, tx, registerID, actor.TenantID, r.outbox); err != nil {
			return err
		}
		if err := linkSettlementToRegisterTx(ctx, tx, r.outbox, settlement, registerID, item.ID, item.AmountWithoutVAT); err != nil {
			return err
		}
		updatedReg, err := getRegisterByIDTx(ctx, tx, registerID, actor.TenantID)
		if err != nil {
			return err
		}
		result.Register = updatedReg
		result.Item = item
		if err := insertRegisterAuditEvent(ctx, tx, actor.TenantID, registerID, domain.RegisterAuditSettlementIncluded, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"settlement_id": settlementID.String(), "register_item_id": item.ID.String(),
			"base_amount": settlement.BaseFreightAmount, "approved_accessorial_total": settlement.ApprovedAccessorialTotal,
		}); err != nil {
			return err
		}
		return insertSettlementAuditEvent(ctx, tx, actor.TenantID, settlementID, "REGISTER_INCLUDED", actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"billing_register_id": registerID.String(), "billing_register_item_id": item.ID.String(),
		})
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *BillingRegisterRepository) RemoveSettlement(
	ctx context.Context,
	registerID, settlementID uuid.UUID,
	actor domain.SettlementActorInput,
) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateBillingRegisterAccess(reg, actor.ActorCompanyID, actor.ActorKind); err != nil {
			return err
		}
		if actor.ActorKind != domain.SettlementActorBuyer {
			return apperrors.Forbidden("only buyer can remove settlement from billing register")
		}
		if err := domain.ValidateDeleteItemRegisterStatus(reg.Status); err != nil {
			return err
		}
		item, err := findRegisterItemBySettlementTx(ctx, tx, registerID, settlementID, actor.TenantID)
		if err != nil {
			return err
		}
		tag, execErr := tx.Exec(ctx, `DELETE FROM billing.billing_register_items WHERE id = $1 AND register_id = $2 AND tenant_id = $3`, item.ID, registerID, actor.TenantID)
		if execErr != nil {
			return mapDBError(execErr)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("settlement not found in register")
		}
		const clear = `
			UPDATE billing.freight_settlements
			SET billing_register_id = NULL, billing_register_item_id = NULL,
				billing_link_revision = billing_link_revision + 1,
				updated_at = now(), version = version + 1
			WHERE id = $1 AND tenant_id = $2 AND billing_register_id = $3
			RETURNING ` + freightSettlementSelectColumns
		unlinked, clearErr := scanSettlement(tx.QueryRow(ctx, clear, settlementID, actor.TenantID, registerID))
		if clearErr != nil {
			return mapDBError(clearErr)
		}
		if err := r.outbox.EmitBillingLinkSnapshotTx(ctx, tx, unlinked, domain.BillingLinkStateUnlinked, nil, nil, nil, time.Now().UTC()); err != nil {
			return err
		}
		if err := recalculateRegisterTotalsTx(ctx, tx, registerID, actor.TenantID, r.outbox); err != nil {
			return err
		}
		updated, err := getRegisterByIDTx(ctx, tx, registerID, actor.TenantID)
		if err != nil {
			return err
		}
		result = updated
		return insertRegisterAuditEvent(ctx, tx, actor.TenantID, registerID, domain.RegisterAuditSettlementRemoved, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"settlement_id": settlementID.String(),
		})
	})
	return result, err
}

func (r *BillingRegisterRepository) withRegisterAuditTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

func (r *BillingRegisterRepository) SimulateRegisterAuditFailureForTest(ctx context.Context, registerID, tenantID uuid.UUID) error {
	return r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, tenantID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE billing.billing_registers SET status = $3, updated_at = now() WHERE id = $1 AND tenant_id = $2`, registerID, tenantID, domain.RegisterStatusCalculated)
		if err != nil {
			return mapDBError(err)
		}
		_ = reg
		return errors.New("simulated register audit failure")
	})
}

func getRegisterByIDTx(ctx context.Context, tx pgx.Tx, id, tenantID uuid.UUID) (*domain.BillingRegister, error) {
	const query = `
		SELECT id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
		FROM billing.billing_registers WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	reg, err := scanRegister(tx.QueryRow(ctx, query, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("billing register not found")
	}
	return reg, err
}

func getSettlementByIDTx(ctx context.Context, tx pgx.Tx, id, tenantID uuid.UUID) (*domain.FreightSettlement, error) {
	query := `
		SELECT ` + freightSettlementSelectColumns + `
		FROM billing.freight_settlements WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	row := tx.QueryRow(ctx, query, id, tenantID)
	settlement, err := scanSettlement(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("freight settlement not found")
	}
	return settlement, err
}

func findRegisterItemBySettlementTx(ctx context.Context, tx pgx.Tx, registerID, settlementID, tenantID uuid.UUID) (*domain.BillingRegisterItem, error) {
	const query = `
		SELECT id, tenant_id, register_id, settlement_id, shipment_id, transport_order_id, route_description,
			pickup_date, delivery_date, shipper_company_id, consignee_company_id, carrier_company_id,
			base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, created_at
		FROM billing.billing_register_items
		WHERE register_id = $1 AND settlement_id = $2 AND tenant_id = $3`
	item, err := scanItemWithSettlement(tx.QueryRow(ctx, query, registerID, settlementID, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("settlement not found in register")
	}
	return item, err
}

func findRegisterItemBySettlementAnyTx(ctx context.Context, tx pgx.Tx, settlementID, tenantID uuid.UUID) (*domain.BillingRegisterItem, error) {
	const query = `
		SELECT id, tenant_id, register_id, settlement_id, shipment_id, transport_order_id, route_description,
			pickup_date, delivery_date, shipper_company_id, consignee_company_id, carrier_company_id,
			base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, created_at
		FROM billing.billing_register_items
		WHERE settlement_id = $1 AND tenant_id = $2`
	item, err := scanItemWithSettlement(tx.QueryRow(ctx, query, settlementID, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func linkSettlementToRegisterTx(
	ctx context.Context,
	tx pgx.Tx,
	emitter *FreightCostOutboxEmitter,
	settlement *domain.FreightSettlement,
	registerID, itemID uuid.UUID,
	amountWithoutVAT float64,
) error {
	const update = `
		UPDATE billing.freight_settlements
		SET billing_register_id = $3, billing_register_item_id = $4,
			billing_link_revision = billing_link_revision + 1,
			updated_at = now(), version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		RETURNING ` + freightSettlementSelectColumns
	linked, err := scanSettlement(tx.QueryRow(ctx, update, settlement.ID, settlement.TenantID, registerID, itemID))
	if err != nil {
		return mapDBError(err)
	}
	amount := domain.FormatMoneyFloat(amountWithoutVAT)
	return emitter.EmitBillingLinkSnapshotTx(ctx, tx, linked, domain.BillingLinkStateLinked, &amount, &registerID, &itemID, time.Now().UTC())
}

func recalculateRegisterTotalsTx(ctx context.Context, tx pgx.Tx, registerID, tenantID uuid.UUID, emitter *FreightCostOutboxEmitter) error {
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
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	if _, err := tx.Exec(ctx, recalc, registerID, tenantID); err != nil {
		return mapDBError(err)
	}
	if emitter == nil {
		return nil
	}
	reg, err := getRegisterByIDTx(ctx, tx, registerID, tenantID)
	if err != nil {
		return err
	}
	return emitter.EmitPayableSnapshotTx(ctx, tx, reg, time.Now().UTC())
}

func countOpenSettlementDisputesTx(ctx context.Context, tx pgx.Tx, settlementID, tenantID uuid.UUID) (int, error) {
	const query = `SELECT COUNT(*) FROM billing.settlement_disputes WHERE settlement_id = $1 AND tenant_id = $2 AND status = $3`
	var count int
	err := tx.QueryRow(ctx, query, settlementID, tenantID, domain.DisputeStatusOpen).Scan(&count)
	return count, mapDBError(err)
}

func insertRegisterAuditEvent(ctx context.Context, tx pgx.Tx, tenantID, registerID uuid.UUID, eventType string, actorUser, actorCompany uuid.UUID, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return apperrors.Internal("marshal audit payload", err)
	}
	const insert = `
		INSERT INTO billing.billing_register_audit_events (tenant_id, register_id, event_type, actor_user_id, actor_company_id, payload)
		VALUES ($1,$2,$3,$4,$5,$6)`
	_, err = tx.Exec(ctx, insert, tenantID, registerID, eventType, actorUser, actorCompany, raw)
	return mapDBError(err)
}

func insertSettlementAuditEvent(ctx context.Context, tx pgx.Tx, tenantID, settlementID uuid.UUID, eventType string, actorUser, actorCompany uuid.UUID, payload map[string]any) error {
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

func scanItemWithSettlement(row scannable) (*domain.BillingRegisterItem, error) {
	var item domain.BillingRegisterItem
	var settlementID *uuid.UUID
	err := row.Scan(
		&item.ID, &item.TenantID, &item.RegisterID, &settlementID, &item.ShipmentID, &item.TransportOrderID, &item.RouteDescription,
		&item.PickupDate, &item.DeliveryDate, &item.ShipperCompanyID, &item.ConsigneeCompanyID, &item.CarrierCompanyID,
		&item.BaseAmount, &item.ExtraCharges, &item.Penalties,
		&item.AmountWithoutVAT, &item.VATRate, &item.VATAmount, &item.AmountWithVAT, &item.Status, &item.CreatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	item.SettlementID = settlementID
	return &item, nil
}
