package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

func (r *BillingRegisterRepository) CreateWithAudit(ctx context.Context, in domain.CreateBillingRegisterInput, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		const query = `
			INSERT INTO billing.billing_registers (
				tenant_id, register_number, customer_company_id, contractor_company_id,
				contract_id, period_from, period_to, currency_code, vat_rate, status, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
				contract_id, period_from, period_to, currency_code, vat_rate, status,
				total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version`
		reg, err := scanRegister(tx.QueryRow(ctx, query,
			actor.TenantID, in.RegisterNumber, in.CustomerCompanyID, in.ContractorCompanyID,
			optionalUUID(in.ContractID), in.PeriodFrom, in.PeriodTo,
			domain.NormalizeCurrencyCode(in.CurrencyCode), optionalFloat(in.VATRate), domain.RegisterStatusDraft, actor.ActorUserID,
		))
		if err != nil {
			return err
		}
		result = reg
		return insertRegisterAuditEvent(ctx, tx, actor.TenantID, reg.ID, domain.RegisterAuditCreated, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"register_number": reg.RegisterNumber, "status": reg.Status,
		})
	})
	return result, err
}

func (r *BillingRegisterRepository) CalculateForActor(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
			return err
		}
		if reg.Status == domain.RegisterStatusCalculated {
			result = reg
			return nil
		}
		if err := domain.ValidateCalculateRegisterStatus(reg.Status); err != nil {
			return err
		}
		totals, err := sumItemTotalsTx(ctx, tx, registerID)
		if err != nil {
			return err
		}
		const query = `
			UPDATE billing.billing_registers
			SET total_without_vat = $1, vat_amount = $2, total_with_vat = $3,
				status = $4, version = version + 1, updated_at = now()
			WHERE id = $5 AND tenant_id = $6 AND deleted_at IS NULL AND version = $7
			RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
				contract_id, period_from, period_to, currency_code, vat_rate, status,
				total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version`
		updated, err := scanRegister(tx.QueryRow(ctx, query,
			totals.TotalWithoutVAT, totals.VATAmount, totals.TotalWithVAT,
			domain.RegisterStatusCalculated, registerID, actor.TenantID, reg.Version,
		))
		if err != nil {
			return err
		}
		result = updated
		return insertRegisterAuditEvent(ctx, tx, actor.TenantID, registerID, domain.RegisterAuditCalculated, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"from": reg.Status, "to": updated.Status,
			"total_without_vat": updated.TotalWithoutVAT, "total_with_vat": updated.TotalWithVAT,
		})
	})
	return result, err
}

func (r *BillingRegisterRepository) ApproveForActor(ctx context.Context, registerID uuid.UUID, actor domain.SettlementActorInput) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
			return err
		}
		if reg.Status == domain.RegisterStatusApproved {
			result = reg
			return nil
		}
		if err := domain.ValidateApproveRegisterStatus(reg.Status); err != nil {
			return err
		}
		if err := domain.ValidateApproveRegisterTotals(reg.TotalWithVAT); err != nil {
			return err
		}
		const query = `
			UPDATE billing.billing_registers
			SET status = $1, approved_at = now(), approved_by = $2, version = version + 1, updated_at = now()
			WHERE id = $3 AND tenant_id = $4 AND deleted_at IS NULL AND version = $5
			RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
				contract_id, period_from, period_to, currency_code, vat_rate, status,
				total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version`
		updated, err := scanRegister(tx.QueryRow(ctx, query, domain.RegisterStatusApproved, actor.ActorUserID, registerID, actor.TenantID, reg.Version))
		if err != nil {
			return err
		}
		result = updated
		return insertRegisterAuditEvent(ctx, tx, actor.TenantID, registerID, domain.RegisterAuditApproved, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"from": reg.Status, "to": updated.Status, "approved_by": actor.ActorUserID.String(),
		})
	})
	return result, err
}

func (r *BillingRegisterRepository) TransitionStatusForActor(
	ctx context.Context,
	registerID uuid.UUID,
	actor domain.SettlementActorInput,
	nextStatus, auditEvent string,
	validate func(string) error,
) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, actor.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
			return err
		}
		if reg.Status == nextStatus {
			result = reg
			return nil
		}
		if err := validate(reg.Status); err != nil {
			return err
		}
		const query = `
			UPDATE billing.billing_registers
			SET status = $1, version = version + 1, updated_at = now()
			WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $4
			RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
				contract_id, period_from, period_to, currency_code, vat_rate, status,
				total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version`
		updated, err := scanRegister(tx.QueryRow(ctx, query, nextStatus, registerID, actor.TenantID, reg.Version))
		if err != nil {
			return err
		}
		result = updated
		return insertRegisterAuditEvent(ctx, tx, actor.TenantID, registerID, auditEvent, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"from": reg.Status, "to": updated.Status,
		})
	})
	return result, err
}

func sumItemTotalsTx(ctx context.Context, tx pgx.Tx, registerID uuid.UUID) (registerTotals, error) {
	const query = `
		SELECT COALESCE(SUM(amount_without_vat),0), COALESCE(SUM(vat_amount),0), COALESCE(SUM(amount_with_vat),0)
		FROM billing.billing_register_items WHERE register_id = $1`
	var totals registerTotals
	if err := tx.QueryRow(ctx, query, registerID).Scan(&totals.TotalWithoutVAT, &totals.VATAmount, &totals.TotalWithVAT); err != nil {
		return registerTotals{}, mapDBError(err)
	}
	return totals, nil
}

func (r *BillingRegisterRepository) SimulateCalculateAuditFailureForTest(ctx context.Context, registerID, tenantID uuid.UUID) error {
	return r.withRegisterAuditTx(ctx, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, tenantID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE billing.billing_registers SET status = $3, updated_at = now()
			WHERE id = $1 AND tenant_id = $2`, registerID, tenantID, domain.RegisterStatusCalculated); err != nil {
			return mapDBError(err)
		}
		_ = reg
		return apperrors.Internal("simulated calculate audit failure", nil)
	})
}
