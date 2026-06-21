package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

type ClosingDocumentRepository struct {
	pool *pgxpool.Pool
}

func NewClosingDocumentRepository(pool *pgxpool.Pool) *ClosingDocumentRepository {
	return &ClosingDocumentRepository{pool: pool}
}

func (r *ClosingDocumentRepository) CreatePackage(ctx context.Context, registerID uuid.UUID, in domain.CreateClosingDocumentPackageInput) (*domain.ClosingDocumentPackage, error) {
	var result *domain.ClosingDocumentPackage
	err := measureDB("closing_document_repository", "create_closing_document_package", func() error {
		const query = `
		INSERT INTO billing.closing_document_packages (tenant_id, register_id, package_number, package_type, status)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, tenant_id, register_id, package_number, package_type, status, created_at
	`
		var p domain.ClosingDocumentPackage
		err := r.pool.QueryRow(ctx, query,
			in.TenantID, registerID, strings.TrimSpace(in.PackageNumber), strings.TrimSpace(in.PackageType), domain.ClosingPackageStatusDraft,
		).Scan(&p.ID, &p.TenantID, &p.RegisterID, &p.PackageNumber, &p.PackageType, &p.Status, &p.CreatedAt)
		if err != nil {
			return mapDBError(err)
		}
		result = &p
		return nil
	})
	return result, err
}

func (r *ClosingDocumentRepository) CreateInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateInvoiceInput) (*domain.Invoice, error) {
	var result *domain.Invoice
	err := measureDB("closing_document_repository", "create_invoice", func() error {
		const query = `
		INSERT INTO billing.invoices (
			tenant_id, register_id, invoice_number, invoice_date,
			seller_company_id, buyer_company_id, total_amount, currency_code, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, tenant_id, register_id, invoice_number, invoice_date, seller_company_id, buyer_company_id,
			total_amount, currency_code, status, document_id, created_at
	`
		var inv domain.Invoice
		err := r.pool.QueryRow(ctx, query,
			in.TenantID, register.ID, strings.TrimSpace(in.InvoiceNumber), in.InvoiceDate,
			in.SellerCompanyID, in.BuyerCompanyID, register.TotalWithVAT, register.CurrencyCode, domain.InvoiceStatusDraft,
		).Scan(&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.InvoiceNumber, &inv.InvoiceDate,
			&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.TotalAmount, &inv.CurrencyCode, &inv.Status, &inv.DocumentID, &inv.CreatedAt)
		if err != nil {
			return mapDBError(err)
		}
		result = &inv
		return nil
	})
	return result, err
}

func (r *ClosingDocumentRepository) CreateAct(ctx context.Context, register *domain.BillingRegister, in domain.CreateActInput) (*domain.Act, error) {
	var result *domain.Act
	err := measureDB("closing_document_repository", "create_act", func() error {
		const query = `
		INSERT INTO billing.acts (
			tenant_id, register_id, act_number, act_date, seller_company_id, buyer_company_id,
			service_description, total_amount, currency_code, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, tenant_id, register_id, act_number, act_date, seller_company_id, buyer_company_id,
			service_description, total_amount, currency_code, status, document_id, created_at
	`
		var act domain.Act
		err := r.pool.QueryRow(ctx, query,
			in.TenantID, register.ID, strings.TrimSpace(in.ActNumber), in.ActDate,
			in.SellerCompanyID, in.BuyerCompanyID, optionalString(in.ServiceDescription),
			register.TotalWithVAT, register.CurrencyCode, domain.ActStatusDraft,
		).Scan(&act.ID, &act.TenantID, &act.RegisterID, &act.ActNumber, &act.ActDate,
			&act.SellerCompanyID, &act.BuyerCompanyID, &act.ServiceDescription, &act.TotalAmount, &act.CurrencyCode, &act.Status, &act.DocumentID, &act.CreatedAt)
		if err != nil {
			return mapDBError(err)
		}
		result = &act
		return nil
	})
	return result, err
}

func (r *ClosingDocumentRepository) CreateVATInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateVATInvoiceInput) (*domain.VATInvoice, error) {
	var result *domain.VATInvoice
	err := measureDB("closing_document_repository", "create_vat_invoice", func() error {
		const query = `
		INSERT INTO billing.vat_invoices (
			tenant_id, register_id, vat_invoice_number, vat_invoice_date,
			seller_company_id, buyer_company_id, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, tenant_id, register_id, vat_invoice_number, vat_invoice_date, seller_company_id, buyer_company_id,
			amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at
	`
		var inv domain.VATInvoice
		err := r.pool.QueryRow(ctx, query,
			in.TenantID, register.ID, strings.TrimSpace(in.VATInvoiceNumber), in.VATInvoiceDate,
			in.SellerCompanyID, in.BuyerCompanyID, register.TotalWithoutVAT, optionalFloat(register.VATRate),
			register.VATAmount, register.TotalWithVAT, domain.VATInvoiceStatusDraft,
		).Scan(&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.VATInvoiceNumber, &inv.VATInvoiceDate,
			&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.AmountWithoutVAT, &inv.VATRate, &inv.VATAmount, &inv.AmountWithVAT, &inv.Status, &inv.DocumentID, &inv.CreatedAt)
		if err != nil {
			return mapDBError(err)
		}
		result = &inv
		return nil
	})
	return result, err
}

func (r *ClosingDocumentRepository) CreateUPD(ctx context.Context, register *domain.BillingRegister, in domain.CreateUPDInput) (*domain.UPDDocument, error) {
	var result *domain.UPDDocument
	err := measureDB("closing_document_repository", "create_upd", func() error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return mapDBError(err)
		}
		defer tx.Rollback(ctx)

		const query = `
		INSERT INTO billing.upd_documents (
			tenant_id, register_id, upd_number, upd_date, seller_company_id, buyer_company_id,
			function_code, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, tenant_id, register_id, upd_number, upd_date, seller_company_id, buyer_company_id,
			function_code, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at
	`
		var upd domain.UPDDocument
		err = tx.QueryRow(ctx, query,
			in.TenantID, register.ID, strings.TrimSpace(in.UPDNumber), in.UPDDate,
			in.SellerCompanyID, in.BuyerCompanyID, strings.TrimSpace(in.FunctionCode),
			register.TotalWithoutVAT, optionalFloat(register.VATRate), register.VATAmount, register.TotalWithVAT, domain.UPDStatusDraft,
		).Scan(&upd.ID, &upd.TenantID, &upd.RegisterID, &upd.UPDNumber, &upd.UPDDate,
			&upd.SellerCompanyID, &upd.BuyerCompanyID, &upd.FunctionCode, &upd.AmountWithoutVAT, &upd.VATRate, &upd.VATAmount, &upd.AmountWithVAT, &upd.Status, &upd.DocumentID, &upd.CreatedAt)
		if err != nil {
			return mapDBError(err)
		}

		_, err = tx.Exec(ctx, `
		UPDATE billing.billing_registers
		SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL
	`, domain.RegisterStatusClosingDocumentsCreated, register.ID, in.TenantID)
		if err != nil {
			return mapDBError(err)
		}

		if err := tx.Commit(ctx); err != nil {
			return mapDBError(err)
		}
		result = &upd
		return nil
	})
	return result, err
}
