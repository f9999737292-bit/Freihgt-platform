package repository

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

type ClosingDocumentRepository struct {
	pool *pgxpool.Pool
}

func NewClosingDocumentRepository(pool *pgxpool.Pool) *ClosingDocumentRepository {
	return &ClosingDocumentRepository{pool: pool}
}

func (r *ClosingDocumentRepository) CreateInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateInvoiceInput, actor domain.SettlementActorInput) (*domain.Invoice, error) {
	var result *domain.Invoice
	err := withRegisterAuditPoolTx(ctx, r.pool, func(tx pgx.Tx) error {
		number := strings.TrimSpace(in.InvoiceNumber)
		if number == "" {
			number = register.RegisterNumber + "-INV"
		}
		inv, err := ensureInvoiceTx(ctx, tx, register, number, in.InvoiceDate, in.SellerCompanyID, in.BuyerCompanyID, in.TenantID, actor, true)
		if err != nil {
			return err
		}
		result = inv
		return nil
	})
	return result, err
}

func (r *ClosingDocumentRepository) CreateAct(ctx context.Context, register *domain.BillingRegister, in domain.CreateActInput, actor domain.SettlementActorInput) (*domain.Act, error) {
	var result *domain.Act
	err := withRegisterAuditPoolTx(ctx, r.pool, func(tx pgx.Tx) error {
		number := strings.TrimSpace(in.ActNumber)
		if number == "" {
			number = register.RegisterNumber + "-ACT"
		}
		act, err := ensureActTx(ctx, tx, register, number, in.ActDate, in.SellerCompanyID, in.BuyerCompanyID, in.TenantID, actor, true)
		if err != nil {
			return err
		}
		result = act
		return nil
	})
	return result, err
}

func (r *ClosingDocumentRepository) CreateVATInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateVATInvoiceInput, actor domain.SettlementActorInput) (*domain.VATInvoice, error) {
	var result *domain.VATInvoice
	err := withRegisterAuditPoolTx(ctx, r.pool, func(tx pgx.Tx) error {
		number := strings.TrimSpace(in.VATInvoiceNumber)
		if number == "" {
			number = register.RegisterNumber + "-VAT"
		}
		inv, err := ensureVATInvoiceTx(ctx, tx, register, number, in.VATInvoiceDate, in.SellerCompanyID, in.BuyerCompanyID, in.TenantID, actor, true)
		if err != nil {
			return err
		}
		result = inv
		return nil
	})
	return result, err
}

func (r *ClosingDocumentRepository) CreateUPD(ctx context.Context, register *domain.BillingRegister, in domain.CreateUPDInput, actor domain.SettlementActorInput) (*domain.UPDDocument, error) {
	var result *domain.UPDDocument
	err := withRegisterAuditPoolTx(ctx, r.pool, func(tx pgx.Tx) error {
		number := strings.TrimSpace(in.UPDNumber)
		if number == "" {
			number = register.RegisterNumber + "-UPD"
		}
		docDate := in.UPDDate
		if docDate.IsZero() {
			docDate = time.Now().UTC()
		}
		upd, err := ensureUPDTx(ctx, tx, register, number, docDate, in.SellerCompanyID, in.BuyerCompanyID, in.TenantID, actor, true)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE billing.billing_registers
			SET status = $1, version = version + 1, updated_at = now()
			WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND status = $4`,
			domain.RegisterStatusClosingDocumentsCreated, register.ID, in.TenantID, domain.RegisterStatusApproved); err != nil {
			return mapDBError(err)
		}
		result = upd
		return nil
	})
	return result, err
}
