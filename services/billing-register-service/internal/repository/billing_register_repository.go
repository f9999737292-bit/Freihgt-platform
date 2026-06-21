package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type BillingRegisterRepository struct {
	pool *pgxpool.Pool
}

func NewBillingRegisterRepository(pool *pgxpool.Pool) *BillingRegisterRepository {
	return &BillingRegisterRepository{pool: pool}
}

type RegisterDetail struct {
	Register                 *domain.BillingRegister
	Items                    []domain.BillingRegisterItem
	ClosingDocumentPackages  []domain.ClosingDocumentPackage
	Invoices                 []domain.Invoice
	Acts                     []domain.Act
	VATInvoices              []domain.VATInvoice
	UPDDocuments             []domain.UPDDocument
}

func (r *BillingRegisterRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM core.companies WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL)`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *BillingRegisterRepository) GetShipmentStatus(ctx context.Context, shipmentID, tenantID uuid.UUID) (string, error) {
	const query = `SELECT status FROM transport.shipments WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var status string
	err := r.pool.QueryRow(ctx, query, shipmentID, tenantID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperrors.NotFound("shipment not found")
	}
	if err != nil {
		return "", mapDBError(err)
	}
	return status, nil
}

func (r *BillingRegisterRepository) Create(ctx context.Context, in domain.CreateBillingRegisterInput) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := measureDB("billing_register_repository", "create_billing_register", func() error {
		const query = `
		INSERT INTO billing.billing_registers (
			tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
	`
		reg, err := scanRegister(r.pool.QueryRow(ctx, query,
			in.TenantID, strings.TrimSpace(in.RegisterNumber), in.CustomerCompanyID, in.ContractorCompanyID,
			optionalUUID(in.ContractID), in.PeriodFrom, in.PeriodTo,
			domain.NormalizeCurrencyCode(in.CurrencyCode), optionalFloat(in.VATRate), domain.RegisterStatusDraft,
		))
		if err != nil {
			return err
		}
		result = reg
		return nil
	})
	return result, err
}

func (r *BillingRegisterRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := measureDB("billing_register_repository", "get_billing_register", func() error {
		const query = `
		SELECT id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
		FROM billing.billing_registers WHERE id = $1 AND deleted_at IS NULL
	`
		reg, err := scanRegister(r.pool.QueryRow(ctx, query, id))
		if err != nil {
			return err
		}
		result = reg
		return nil
	})
	return result, err
}

func (r *BillingRegisterRepository) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.BillingRegister, error) {
	const query = `
		SELECT id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
		FROM billing.billing_registers WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	reg, err := scanRegister(r.pool.QueryRow(ctx, query, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("billing register not found")
	}
	return reg, err
}

func (r *BillingRegisterRepository) GetDetail(ctx context.Context, id uuid.UUID) (*RegisterDetail, error) {
	reg, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	items, err := r.ListItems(ctx, id, reg.TenantID)
	if err != nil {
		return nil, err
	}
	packages, err := r.listPackages(ctx, id)
	if err != nil {
		return nil, err
	}
	invoices, err := r.listInvoices(ctx, id)
	if err != nil {
		return nil, err
	}
	acts, err := r.listActs(ctx, id)
	if err != nil {
		return nil, err
	}
	vatInvoices, err := r.listVATInvoices(ctx, id)
	if err != nil {
		return nil, err
	}
	upds, err := r.listUPDs(ctx, id)
	if err != nil {
		return nil, err
	}
	return &RegisterDetail{
		Register: reg, Items: items, ClosingDocumentPackages: packages,
		Invoices: invoices, Acts: acts, VATInvoices: vatInvoices, UPDDocuments: upds,
	}, nil
}

func (r *BillingRegisterRepository) List(ctx context.Context, filter domain.ListBillingRegistersFilter) ([]domain.BillingRegister, int, error) {
	var result []domain.BillingRegister
	var total int
	err := measureDB("billing_register_repository", "list_billing_registers", func() error {
		args := []any{filter.TenantID}
		where := []string{"tenant_id = $1", "deleted_at IS NULL"}
		if filter.CustomerCompanyID != nil {
			args = append(args, *filter.CustomerCompanyID)
			where = append(where, fmt.Sprintf("customer_company_id = $%d", len(args)))
		}
		if filter.ContractorCompanyID != nil {
			args = append(args, *filter.ContractorCompanyID)
			where = append(where, fmt.Sprintf("contractor_company_id = $%d", len(args)))
		}
		if filter.Status != nil {
			args = append(args, *filter.Status)
			where = append(where, fmt.Sprintf("status = $%d", len(args)))
		}
		if filter.PeriodFrom != nil {
			args = append(args, *filter.PeriodFrom)
			where = append(where, fmt.Sprintf("period_from >= $%d", len(args)))
		}
		if filter.PeriodTo != nil {
			args = append(args, *filter.PeriodTo)
			where = append(where, fmt.Sprintf("period_to <= $%d", len(args)))
		}
		whereClause := strings.Join(where, " AND ")
		if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM billing.billing_registers WHERE %s", whereClause), args...).Scan(&total); err != nil {
			return mapDBError(err)
		}
		args = append(args, filter.Limit, filter.Offset)
		rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
		FROM billing.billing_registers WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, whereClause, len(args)-1, len(args)), args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()
		regs := make([]domain.BillingRegister, 0)
		for rows.Next() {
			reg, err := scanRegisterRows(rows)
			if err != nil {
				return err
			}
			regs = append(regs, *reg)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		result = regs
		return nil
	})
	return result, total, err
}

func (r *BillingRegisterRepository) AddItem(ctx context.Context, registerID uuid.UUID, amounts domain.ItemAmounts, in domain.CreateBillingRegisterItemInput) (*domain.BillingRegisterItem, error) {
	var result *domain.BillingRegisterItem
	err := measureDB("billing_register_repository", "add_billing_register_item", func() error {
		const query = `
		INSERT INTO billing.billing_register_items (
			tenant_id, register_id, shipment_id, transport_order_id, route_description,
			pickup_date, delivery_date, shipper_company_id, consignee_company_id, carrier_company_id,
			base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id, tenant_id, register_id, shipment_id, transport_order_id, route_description,
			pickup_date, delivery_date, shipper_company_id, consignee_company_id, carrier_company_id,
			base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, created_at
	`
		item, err := scanItem(r.pool.QueryRow(ctx, query,
			in.TenantID, registerID, in.ShipmentID, optionalUUID(in.TransportOrderID), optionalString(in.RouteDescription),
			optionalDate(in.PickupDate), optionalDate(in.DeliveryDate),
			optionalUUID(in.ShipperCompanyID), optionalUUID(in.ConsigneeCompanyID), optionalUUID(in.CarrierCompanyID),
			in.BaseAmount, in.ExtraCharges, in.Penalties,
			amounts.AmountWithoutVAT, optionalFloat(in.VATRate), amounts.VATAmount, amounts.AmountWithVAT,
			domain.RegisterItemStatusDraft,
		))
		if err != nil {
			return err
		}
		result = item
		return nil
	})
	return result, err
}

func (r *BillingRegisterRepository) ListItems(ctx context.Context, registerID, tenantID uuid.UUID) ([]domain.BillingRegisterItem, error) {
	var result []domain.BillingRegisterItem
	err := measureDB("billing_register_repository", "list_billing_register_items", func() error {
		const query = `
		SELECT id, tenant_id, register_id, shipment_id, transport_order_id, route_description,
			pickup_date, delivery_date, shipper_company_id, consignee_company_id, carrier_company_id,
			base_amount, extra_charges, penalties, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, created_at
		FROM billing.billing_register_items WHERE register_id = $1 AND tenant_id = $2 ORDER BY created_at ASC
	`
		rows, err := r.pool.Query(ctx, query, registerID, tenantID)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()
		items := make([]domain.BillingRegisterItem, 0)
		for rows.Next() {
			item, err := scanItemRows(rows)
			if err != nil {
				return err
			}
			items = append(items, *item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		result = items
		return nil
	})
	return result, err
}

func (r *BillingRegisterRepository) DeleteItem(ctx context.Context, registerID, itemID, tenantID uuid.UUID) error {
	return measureDB("billing_register_repository", "delete_billing_register_item", func() error {
		tag, err := r.pool.Exec(ctx, `
		DELETE FROM billing.billing_register_items
		WHERE id = $1 AND register_id = $2 AND tenant_id = $3
	`, itemID, registerID, tenantID)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("billing register item not found")
		}
		return nil
	})
}

func (r *BillingRegisterRepository) Calculate(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := measureDB("billing_register_repository", "calculate_billing_register", func() error {
		totals, err := r.sumItemTotals(ctx, id)
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
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
	`
		reg, err := scanRegisterUpdate(r.pool.QueryRow(ctx, query,
			totals.TotalWithoutVAT, totals.VATAmount, totals.TotalWithVAT,
			domain.RegisterStatusCalculated, id, tenantID, expectedVersion,
		))
		if err != nil {
			return err
		}
		result = reg
		return nil
	})
	return result, err
}

func (r *BillingRegisterRepository) RecalculateAfterItemChange(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error) {
	totals, err := r.sumItemTotals(ctx, id)
	if err != nil {
		return nil, err
	}
	const query = `
		UPDATE billing.billing_registers
		SET total_without_vat = $1, vat_amount = $2, total_with_vat = $3,
			status = $4, version = version + 1, updated_at = now()
		WHERE id = $5 AND tenant_id = $6 AND deleted_at IS NULL AND version = $7
		RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
	`
	return scanRegisterUpdate(r.pool.QueryRow(ctx, query,
		totals.TotalWithoutVAT, totals.VATAmount, totals.TotalWithVAT,
		domain.RegisterStatusDraft, id, tenantID, expectedVersion,
	))
}

func (r *BillingRegisterRepository) Approve(ctx context.Context, id, tenantID, approvedBy uuid.UUID, expectedVersion int) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := measureDB("billing_register_repository", "approve_billing_register", func() error {
		const query = `
		UPDATE billing.billing_registers
		SET status = $1, approved_at = now(), approved_by = $2, version = version + 1, updated_at = now()
		WHERE id = $3 AND tenant_id = $4 AND deleted_at IS NULL AND version = $5
		RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
	`
		reg, err := scanRegisterUpdate(r.pool.QueryRow(ctx, query, domain.RegisterStatusApproved, approvedBy, id, tenantID, expectedVersion))
		if err != nil {
			return err
		}
		result = reg
		return nil
	})
	return result, err
}

func billingRegisterStatusOperation(status string) string {
	switch status {
	case domain.RegisterStatusSentToEDO:
		return "mark_sent_to_edo"
	case domain.RegisterStatusSignedByCounterparty:
		return "mark_signed"
	case domain.RegisterStatusPaid:
		return "mark_paid"
	case domain.RegisterStatusClosed:
		return "close_billing_register"
	default:
		return "update_billing_register_status"
	}
}

func (r *BillingRegisterRepository) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.BillingRegister, error) {
	var result *domain.BillingRegister
	err := measureDB("billing_register_repository", billingRegisterStatusOperation(status), func() error {
		const query = `
		UPDATE billing.billing_registers
		SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $4
		RETURNING id, tenant_id, register_number, customer_company_id, contractor_company_id,
			contract_id, period_from, period_to, currency_code, vat_rate, status,
			total_without_vat, vat_amount, total_with_vat, created_at, approved_at, approved_by, updated_at, version
	`
		reg, err := scanRegisterUpdate(r.pool.QueryRow(ctx, query, status, id, tenantID, expectedVersion))
		if err != nil {
			return err
		}
		result = reg
		return nil
	})
	return result, err
}

type registerTotals struct {
	TotalWithoutVAT float64
	VATAmount       float64
	TotalWithVAT    float64
}

func (r *BillingRegisterRepository) sumItemTotals(ctx context.Context, registerID uuid.UUID) (registerTotals, error) {
	const query = `
		SELECT COALESCE(SUM(amount_without_vat),0), COALESCE(SUM(vat_amount),0), COALESCE(SUM(amount_with_vat),0)
		FROM billing.billing_register_items WHERE register_id = $1
	`
	var totals registerTotals
	if err := r.pool.QueryRow(ctx, query, registerID).Scan(&totals.TotalWithoutVAT, &totals.VATAmount, &totals.TotalWithVAT); err != nil {
		return registerTotals{}, mapDBError(err)
	}
	return totals, nil
}

func (r *BillingRegisterRepository) listPackages(ctx context.Context, registerID uuid.UUID) ([]domain.ClosingDocumentPackage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, register_id, package_number, package_type, status, created_at
		FROM billing.closing_document_packages WHERE register_id = $1 ORDER BY created_at ASC
	`, registerID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanPackages(rows)
}

func (r *BillingRegisterRepository) listInvoices(ctx context.Context, registerID uuid.UUID) ([]domain.Invoice, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, register_id, invoice_number, invoice_date, seller_company_id, buyer_company_id,
			total_amount, currency_code, status, document_id, created_at
		FROM billing.invoices WHERE register_id = $1 ORDER BY created_at ASC
	`, registerID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanInvoices(rows)
}

func (r *BillingRegisterRepository) listActs(ctx context.Context, registerID uuid.UUID) ([]domain.Act, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, register_id, act_number, act_date, seller_company_id, buyer_company_id,
			service_description, total_amount, currency_code, status, document_id, created_at
		FROM billing.acts WHERE register_id = $1 ORDER BY created_at ASC
	`, registerID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanActs(rows)
}

func (r *BillingRegisterRepository) listVATInvoices(ctx context.Context, registerID uuid.UUID) ([]domain.VATInvoice, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, register_id, vat_invoice_number, vat_invoice_date, seller_company_id, buyer_company_id,
			amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at
		FROM billing.vat_invoices WHERE register_id = $1 ORDER BY created_at ASC
	`, registerID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanVATInvoices(rows)
}

func (r *BillingRegisterRepository) listUPDs(ctx context.Context, registerID uuid.UUID) ([]domain.UPDDocument, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, register_id, upd_number, upd_date, seller_company_id, buyer_company_id,
			function_code, amount_without_vat, vat_rate, vat_amount, amount_with_vat, status, document_id, created_at
		FROM billing.upd_documents WHERE register_id = $1 ORDER BY created_at ASC
	`, registerID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	return scanUPDs(rows)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRegister(row rowScanner) (*domain.BillingRegister, error) {
	var reg domain.BillingRegister
	err := row.Scan(
		&reg.ID, &reg.TenantID, &reg.RegisterNumber, &reg.CustomerCompanyID, &reg.ContractorCompanyID,
		&reg.ContractID, &reg.PeriodFrom, &reg.PeriodTo, &reg.CurrencyCode, &reg.VATRate, &reg.Status,
		&reg.TotalWithoutVAT, &reg.VATAmount, &reg.TotalWithVAT,
		&reg.CreatedAt, &reg.ApprovedAt, &reg.ApprovedBy, &reg.UpdatedAt, &reg.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("billing register not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &reg, nil
}

func scanRegisterRows(rows pgx.Rows) (*domain.BillingRegister, error) {
	var reg domain.BillingRegister
	err := rows.Scan(
		&reg.ID, &reg.TenantID, &reg.RegisterNumber, &reg.CustomerCompanyID, &reg.ContractorCompanyID,
		&reg.ContractID, &reg.PeriodFrom, &reg.PeriodTo, &reg.CurrencyCode, &reg.VATRate, &reg.Status,
		&reg.TotalWithoutVAT, &reg.VATAmount, &reg.TotalWithVAT,
		&reg.CreatedAt, &reg.ApprovedAt, &reg.ApprovedBy, &reg.UpdatedAt, &reg.Version,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &reg, nil
}

func scanRegisterUpdate(row rowScanner) (*domain.BillingRegister, error) {
	reg, err := scanRegister(row)
	if err != nil {
		return nil, err
	}
	return reg, nil
}

func scanItem(row rowScanner) (*domain.BillingRegisterItem, error) {
	var item domain.BillingRegisterItem
	err := row.Scan(
		&item.ID, &item.TenantID, &item.RegisterID, &item.ShipmentID, &item.TransportOrderID, &item.RouteDescription,
		&item.PickupDate, &item.DeliveryDate, &item.ShipperCompanyID, &item.ConsigneeCompanyID, &item.CarrierCompanyID,
		&item.BaseAmount, &item.ExtraCharges, &item.Penalties,
		&item.AmountWithoutVAT, &item.VATRate, &item.VATAmount, &item.AmountWithVAT, &item.Status, &item.CreatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &item, nil
}

func scanItemRows(rows pgx.Rows) (*domain.BillingRegisterItem, error) {
	var item domain.BillingRegisterItem
	err := rows.Scan(
		&item.ID, &item.TenantID, &item.RegisterID, &item.ShipmentID, &item.TransportOrderID, &item.RouteDescription,
		&item.PickupDate, &item.DeliveryDate, &item.ShipperCompanyID, &item.ConsigneeCompanyID, &item.CarrierCompanyID,
		&item.BaseAmount, &item.ExtraCharges, &item.Penalties,
		&item.AmountWithoutVAT, &item.VATRate, &item.VATAmount, &item.AmountWithVAT, &item.Status, &item.CreatedAt,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	return &item, nil
}

func scanPackages(rows pgx.Rows) ([]domain.ClosingDocumentPackage, error) {
	packages := make([]domain.ClosingDocumentPackage, 0)
	for rows.Next() {
		var p domain.ClosingDocumentPackage
		if err := rows.Scan(&p.ID, &p.TenantID, &p.RegisterID, &p.PackageNumber, &p.PackageType, &p.Status, &p.CreatedAt); err != nil {
			return nil, mapDBError(err)
		}
		packages = append(packages, p)
	}
	return packages, rows.Err()
}

func scanInvoices(rows pgx.Rows) ([]domain.Invoice, error) {
	items := make([]domain.Invoice, 0)
	for rows.Next() {
		var inv domain.Invoice
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.InvoiceNumber, &inv.InvoiceDate,
			&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.TotalAmount, &inv.CurrencyCode, &inv.Status, &inv.DocumentID, &inv.CreatedAt); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, inv)
	}
	return items, rows.Err()
}

func scanActs(rows pgx.Rows) ([]domain.Act, error) {
	items := make([]domain.Act, 0)
	for rows.Next() {
		var act domain.Act
		if err := rows.Scan(&act.ID, &act.TenantID, &act.RegisterID, &act.ActNumber, &act.ActDate,
			&act.SellerCompanyID, &act.BuyerCompanyID, &act.ServiceDescription, &act.TotalAmount, &act.CurrencyCode, &act.Status, &act.DocumentID, &act.CreatedAt); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, act)
	}
	return items, rows.Err()
}

func scanVATInvoices(rows pgx.Rows) ([]domain.VATInvoice, error) {
	items := make([]domain.VATInvoice, 0)
	for rows.Next() {
		var inv domain.VATInvoice
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.RegisterID, &inv.VATInvoiceNumber, &inv.VATInvoiceDate,
			&inv.SellerCompanyID, &inv.BuyerCompanyID, &inv.AmountWithoutVAT, &inv.VATRate, &inv.VATAmount, &inv.AmountWithVAT, &inv.Status, &inv.DocumentID, &inv.CreatedAt); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, inv)
	}
	return items, rows.Err()
}

func scanUPDs(rows pgx.Rows) ([]domain.UPDDocument, error) {
	items := make([]domain.UPDDocument, 0)
	for rows.Next() {
		var upd domain.UPDDocument
		if err := rows.Scan(&upd.ID, &upd.TenantID, &upd.RegisterID, &upd.UPDNumber, &upd.UPDDate,
			&upd.SellerCompanyID, &upd.BuyerCompanyID, &upd.FunctionCode, &upd.AmountWithoutVAT, &upd.VATRate, &upd.VATAmount, &upd.AmountWithVAT, &upd.Status, &upd.DocumentID, &upd.CreatedAt); err != nil {
			return nil, mapDBError(err)
		}
		items = append(items, upd)
	}
	return items, rows.Err()
}
