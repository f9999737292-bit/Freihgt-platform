package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

func isUniqueViolation(err error) bool {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == apperrors.CodeConflict
	}
	return false
}

func findInvoiceByRegisterTx(ctx context.Context, tx pgx.Tx, registerID, tenantID uuid.UUID) (*domain.Invoice, error) {
	const query = `
		SELECT id, tenant_id, register_id, invoice_number, invoice_date, seller_company_id, buyer_company_id,
			total_amount, currency_code, status, document_id, created_at
		FROM billing.invoices WHERE register_id = $1 AND tenant_id = $2 LIMIT 1`
	var inv domain.Invoice
	err := tx.QueryRow(ctx, query, registerID, tenantID).Scan(
		&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.InvoiceNumber, &inv.InvoiceDate,
		&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.TotalAmount, &inv.CurrencyCode, &inv.Status, &inv.DocumentID, &inv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &inv, nil
}

func findActByRegisterTx(ctx context.Context, tx pgx.Tx, registerID, tenantID uuid.UUID) (*domain.Act, error) {
	const query = `
		SELECT id, tenant_id, register_id, act_number, act_date, seller_company_id, buyer_company_id,
			service_description, total_amount, currency_code, status, document_id, created_at
		FROM billing.acts WHERE register_id = $1 AND tenant_id = $2 LIMIT 1`
	var act domain.Act
	err := tx.QueryRow(ctx, query, registerID, tenantID).Scan(
		&act.ID, &act.TenantID, &act.RegisterID, &act.ActNumber, &act.ActDate,
		&act.SellerCompanyID, &act.BuyerCompanyID, &act.ServiceDescription, &act.TotalAmount, &act.CurrencyCode, &act.Status, &act.DocumentID, &act.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &act, nil
}

func findVATInvoiceByRegisterTx(ctx context.Context, tx pgx.Tx, registerID, tenantID uuid.UUID) (*domain.VATInvoice, error) {
	const query = `
		SELECT id, tenant_id, register_id, vat_invoice_number, vat_invoice_date, seller_company_id, buyer_company_id,
			amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at
		FROM billing.vat_invoices WHERE register_id = $1 AND tenant_id = $2 LIMIT 1`
	var inv domain.VATInvoice
	err := tx.QueryRow(ctx, query, registerID, tenantID).Scan(
		&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.VATInvoiceNumber, &inv.VATInvoiceDate,
		&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.AmountWithoutVAT, &inv.VATRate, &inv.VATAmount, &inv.AmountWithVAT, &inv.Status, &inv.DocumentID, &inv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &inv, nil
}

func findUPDByRegisterTx(ctx context.Context, tx pgx.Tx, registerID, tenantID uuid.UUID) (*domain.UPDDocument, error) {
	const query = `
		SELECT id, tenant_id, register_id, upd_number, upd_date, seller_company_id, buyer_company_id,
			function_code, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at
		FROM billing.upd_documents WHERE register_id = $1 AND tenant_id = $2 LIMIT 1`
	var upd domain.UPDDocument
	err := tx.QueryRow(ctx, query, registerID, tenantID).Scan(
		&upd.ID, &upd.TenantID, &upd.RegisterID, &upd.UPDNumber, &upd.UPDDate,
		&upd.SellerCompanyID, &upd.BuyerCompanyID, &upd.FunctionCode, &upd.AmountWithoutVAT, &upd.VATRate, &upd.VATAmount, &upd.AmountWithVAT, &upd.Status, &upd.DocumentID, &upd.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &upd, nil
}

func ensureInvoiceTx(
	ctx context.Context, tx pgx.Tx, reg *domain.BillingRegister, number string, docDate time.Time,
	sellerID, buyerID, tenantID uuid.UUID, actor domain.SettlementActorInput, audit bool,
) (*domain.Invoice, error) {
	existing, err := findInvoiceByRegisterTx(ctx, tx, reg.ID, tenantID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	const query = `
		INSERT INTO billing.invoices (
			tenant_id, register_id, invoice_number, invoice_date,
			seller_company_id, buyer_company_id, total_amount, currency_code, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, tenant_id, register_id, invoice_number, invoice_date, seller_company_id, buyer_company_id,
			total_amount, currency_code, status, document_id, created_at`
	var inv domain.Invoice
	err = tx.QueryRow(ctx, query, tenantID, reg.ID, number, docDate, sellerID, buyerID, reg.TotalWithVAT, reg.CurrencyCode, domain.InvoiceStatusDraft).Scan(
		&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.InvoiceNumber, &inv.InvoiceDate,
		&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.TotalAmount, &inv.CurrencyCode, &inv.Status, &inv.DocumentID, &inv.CreatedAt,
	)
	if err != nil {
		if mapped := mapDBError(err); isUniqueViolation(mapped) {
			return findInvoiceByRegisterTx(ctx, tx, reg.ID, tenantID)
		}
		return nil, mapDBError(err)
	}
	if audit && actor.ActorUserID != uuid.Nil {
		if err := insertRegisterAuditEvent(ctx, tx, tenantID, reg.ID, domain.RegisterAuditInvoiceCreated, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"invoice_id": inv.ID.String(), "invoice_number": inv.InvoiceNumber,
		}); err != nil {
			return nil, err
		}
	}
	return &inv, nil
}

func ensureActTx(
	ctx context.Context, tx pgx.Tx, reg *domain.BillingRegister, number string, docDate time.Time,
	sellerID, buyerID, tenantID uuid.UUID, actor domain.SettlementActorInput, audit bool,
) (*domain.Act, error) {
	existing, err := findActByRegisterTx(ctx, tx, reg.ID, tenantID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	const query = `
		INSERT INTO billing.acts (
			tenant_id, register_id, act_number, act_date, seller_company_id, buyer_company_id,
			total_amount, currency_code, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, tenant_id, register_id, act_number, act_date, seller_company_id, buyer_company_id,
			service_description, total_amount, currency_code, status, document_id, created_at`
	var act domain.Act
	err = tx.QueryRow(ctx, query, tenantID, reg.ID, number, docDate, sellerID, buyerID, reg.TotalWithVAT, reg.CurrencyCode, domain.ActStatusDraft).Scan(
		&act.ID, &act.TenantID, &act.RegisterID, &act.ActNumber, &act.ActDate,
		&act.SellerCompanyID, &act.BuyerCompanyID, &act.ServiceDescription, &act.TotalAmount, &act.CurrencyCode, &act.Status, &act.DocumentID, &act.CreatedAt,
	)
	if err != nil {
		if mapped := mapDBError(err); isUniqueViolation(mapped) {
			return findActByRegisterTx(ctx, tx, reg.ID, tenantID)
		}
		return nil, mapDBError(err)
	}
	if audit && actor.ActorUserID != uuid.Nil {
		if err := insertRegisterAuditEvent(ctx, tx, tenantID, reg.ID, domain.RegisterAuditActCreated, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"act_id": act.ID.String(), "act_number": act.ActNumber,
		}); err != nil {
			return nil, err
		}
	}
	return &act, nil
}

func ensureVATInvoiceTx(
	ctx context.Context, tx pgx.Tx, reg *domain.BillingRegister, number string, docDate time.Time,
	sellerID, buyerID, tenantID uuid.UUID, actor domain.SettlementActorInput, audit bool,
) (*domain.VATInvoice, error) {
	existing, err := findVATInvoiceByRegisterTx(ctx, tx, reg.ID, tenantID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	const query = `
		INSERT INTO billing.vat_invoices (
			tenant_id, register_id, vat_invoice_number, vat_invoice_date,
			seller_company_id, buyer_company_id, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, tenant_id, register_id, vat_invoice_number, vat_invoice_date, seller_company_id, buyer_company_id,
			amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at`
	var inv domain.VATInvoice
	err = tx.QueryRow(ctx, query, tenantID, reg.ID, number, docDate, sellerID, buyerID,
		reg.TotalWithoutVAT, optionalFloat(reg.VATRate), reg.VATAmount, reg.TotalWithVAT, domain.VATInvoiceStatusDraft).Scan(
		&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.VATInvoiceNumber, &inv.VATInvoiceDate,
		&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.AmountWithoutVAT, &inv.VATRate, &inv.VATAmount, &inv.AmountWithVAT, &inv.Status, &inv.DocumentID, &inv.CreatedAt,
	)
	if err != nil {
		if mapped := mapDBError(err); isUniqueViolation(mapped) {
			return findVATInvoiceByRegisterTx(ctx, tx, reg.ID, tenantID)
		}
		return nil, mapDBError(err)
	}
	if audit && actor.ActorUserID != uuid.Nil {
		if err := insertRegisterAuditEvent(ctx, tx, tenantID, reg.ID, domain.RegisterAuditVATInvoiceCreated, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"vat_invoice_id": inv.ID.String(), "vat_invoice_number": inv.VATInvoiceNumber,
		}); err != nil {
			return nil, err
		}
	}
	return &inv, nil
}

func ensureUPDTx(
	ctx context.Context, tx pgx.Tx, reg *domain.BillingRegister, number string, docDate time.Time,
	sellerID, buyerID, tenantID uuid.UUID, actor domain.SettlementActorInput, audit bool,
) (*domain.UPDDocument, error) {
	existing, err := findUPDByRegisterTx(ctx, tx, reg.ID, tenantID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	const query = `
		INSERT INTO billing.upd_documents (
			tenant_id, register_id, upd_number, upd_date, seller_company_id, buyer_company_id,
			function_code, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, tenant_id, register_id, upd_number, upd_date, seller_company_id, buyer_company_id,
			function_code, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at`
	var upd domain.UPDDocument
	err = tx.QueryRow(ctx, query, tenantID, reg.ID, number, docDate, sellerID, buyerID, "СЧФДОП",
		reg.TotalWithoutVAT, optionalFloat(reg.VATRate), reg.VATAmount, reg.TotalWithVAT, domain.UPDStatusDraft).Scan(
		&upd.ID, &upd.TenantID, &upd.RegisterID, &upd.UPDNumber, &upd.UPDDate,
		&upd.SellerCompanyID, &upd.BuyerCompanyID, &upd.FunctionCode, &upd.AmountWithoutVAT, &upd.VATRate, &upd.VATAmount, &upd.AmountWithVAT, &upd.Status, &upd.DocumentID, &upd.CreatedAt,
	)
	if err != nil {
		if mapped := mapDBError(err); isUniqueViolation(mapped) {
			return findUPDByRegisterTx(ctx, tx, reg.ID, tenantID)
		}
		return nil, mapDBError(err)
	}
	if audit && actor.ActorUserID != uuid.Nil {
		if err := insertRegisterAuditEvent(ctx, tx, tenantID, reg.ID, domain.RegisterAuditUPDCreated, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"upd_id": upd.ID.String(), "upd_number": upd.UPDNumber,
		}); err != nil {
			return nil, err
		}
	}
	return &upd, nil
}
