package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

type ClosingPackageResult struct {
	Package    *domain.ClosingDocumentPackage
	Invoice    *domain.Invoice
	Act        *domain.Act
	VATInvoice *domain.VATInvoice
	UPD        *domain.UPDDocument
}

func (r *ClosingDocumentRepository) CreatePackage(ctx context.Context, registerID uuid.UUID, in domain.CreateClosingDocumentPackageInput) (*domain.ClosingDocumentPackage, error) {
	result, err := r.createPackageWithDocuments(ctx, registerID, in, domain.SettlementActorInput{
		TenantID: in.TenantID,
	})
	if err != nil {
		return nil, err
	}
	return result.Package, nil
}

func (r *ClosingDocumentRepository) CreatePackageForActor(
	ctx context.Context,
	registerID uuid.UUID,
	in domain.CreateClosingDocumentPackageInput,
	actor domain.SettlementActorInput,
) (*ClosingPackageResult, error) {
	return r.createPackageWithDocuments(ctx, registerID, in, actor)
}

func (r *ClosingDocumentRepository) createPackageWithDocuments(
	ctx context.Context,
	registerID uuid.UUID,
	in domain.CreateClosingDocumentPackageInput,
	actor domain.SettlementActorInput,
) (*ClosingPackageResult, error) {
	var result ClosingPackageResult
	err := withRegisterAuditPoolTx(ctx, r.pool, func(tx pgx.Tx) error {
		reg, err := getRegisterByIDTx(ctx, tx, registerID, in.TenantID)
		if err != nil {
			return err
		}
		if err := domain.ValidateRegisterBuyerMutation(reg, actor); err != nil {
			return err
		}
		if err := domain.ValidateCreateClosingDocumentRegisterStatus(reg.Status); err != nil {
			return err
		}

		existing, err := findPackageByRegisterTx(ctx, tx, registerID, in.TenantID)
		if err != nil {
			return err
		}
		if existing != nil {
			result.Package = existing
			return loadPackageDocumentsTx(ctx, tx, registerID, in.TenantID, &result)
		}

		pkgNumber := strings.TrimSpace(in.PackageNumber)
		if pkgNumber == "" {
			pkgNumber = fmt.Sprintf("PKG-%s", reg.RegisterNumber)
		}
		pkgType := strings.TrimSpace(in.PackageType)
		if pkgType == "" {
			pkgType = domain.ClosingPackageTypeActPlusVATInvoice
		}

		const insertPkg = `
			INSERT INTO billing.closing_document_packages (tenant_id, register_id, package_number, package_type, status)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING id, tenant_id, register_id, package_number, package_type, status, created_at`
		var pkg domain.ClosingDocumentPackage
		if err := tx.QueryRow(ctx, insertPkg, in.TenantID, registerID, pkgNumber, pkgType, domain.ClosingPackageStatusDraft).Scan(
			&pkg.ID, &pkg.TenantID, &pkg.RegisterID, &pkg.PackageNumber, &pkg.PackageType, &pkg.Status, &pkg.CreatedAt,
		); err != nil {
			if mapped := mapDBError(err); isUniqueViolation(mapped) {
				existing, findErr := findPackageByRegisterTx(ctx, tx, registerID, in.TenantID)
				if findErr != nil {
					return findErr
				}
				if existing == nil {
					return mapped
				}
				result.Package = existing
				return loadPackageDocumentsTx(ctx, tx, registerID, in.TenantID, &result)
			}
			return mapDBError(err)
		}
		result.Package = &pkg

		docDate := time.Now().UTC()
		sellerID := reg.ContractorCompanyID
		buyerID := reg.CustomerCompanyID
		prefix := reg.RegisterNumber

		switch pkgType {
		case domain.ClosingPackageTypeInvoiceOnly:
			inv, createErr := ensureInvoiceTx(ctx, tx, reg, prefix+"-INV", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.Invoice = inv
		case domain.ClosingPackageTypeUPD:
			upd, createErr := ensureUPDTx(ctx, tx, reg, prefix+"-UPD", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.UPD = upd
		case domain.ClosingPackageTypeCustom:
			inv, createErr := ensureInvoiceTx(ctx, tx, reg, prefix+"-INV", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.Invoice = inv
			act, createErr := ensureActTx(ctx, tx, reg, prefix+"-ACT", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.Act = act
			vat, createErr := ensureVATInvoiceTx(ctx, tx, reg, prefix+"-VAT", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.VATInvoice = vat
			upd, createErr := ensureUPDTx(ctx, tx, reg, prefix+"-UPD", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.UPD = upd
		default: // ACT_PLUS_VAT_INVOICE
			act, createErr := ensureActTx(ctx, tx, reg, prefix+"-ACT", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.Act = act
			vat, createErr := ensureVATInvoiceTx(ctx, tx, reg, prefix+"-VAT", docDate, sellerID, buyerID, in.TenantID, actor, true)
			if createErr != nil {
				return createErr
			}
			result.VATInvoice = vat
		}

		if _, err := tx.Exec(ctx, `
			UPDATE billing.billing_registers
			SET status = $1, version = version + 1, updated_at = now()
			WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL`,
			domain.RegisterStatusClosingDocumentsCreated, registerID, in.TenantID); err != nil {
			return mapDBError(err)
		}

		return insertRegisterAuditEvent(ctx, tx, in.TenantID, registerID, domain.RegisterAuditClosingPackage, actor.ActorUserID, actor.ActorCompanyID, map[string]any{
			"package_id": pkg.ID.String(), "package_number": pkg.PackageNumber, "package_type": pkg.PackageType,
		})
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func findPackageByRegisterTx(ctx context.Context, tx pgx.Tx, registerID, tenantID uuid.UUID) (*domain.ClosingDocumentPackage, error) {
	const query = `
		SELECT id, tenant_id, register_id, package_number, package_type, status, created_at
		FROM billing.closing_document_packages WHERE register_id = $1 AND tenant_id = $2 LIMIT 1`
	var pkg domain.ClosingDocumentPackage
	err := tx.QueryRow(ctx, query, registerID, tenantID).Scan(
		&pkg.ID, &pkg.TenantID, &pkg.RegisterID, &pkg.PackageNumber, &pkg.PackageType, &pkg.Status, &pkg.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return &pkg, nil
}

func loadPackageDocumentsTx(ctx context.Context, tx pgx.Tx, registerID, tenantID uuid.UUID, result *ClosingPackageResult) error {
	inv, err := findInvoiceByRegisterTx(ctx, tx, registerID, tenantID)
	if err != nil {
		return err
	}
	result.Invoice = inv
	act, err := findActByRegisterTx(ctx, tx, registerID, tenantID)
	if err != nil {
		return err
	}
	result.Act = act
	vat, err := findVATInvoiceByRegisterTx(ctx, tx, registerID, tenantID)
	if err != nil {
		return err
	}
	result.VATInvoice = vat
	upd, err := findUPDByRegisterTx(ctx, tx, registerID, tenantID)
	if err != nil {
		return err
	}
	result.UPD = upd
	return nil
}

func withRegisterAuditPoolTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return mapDBError(err)
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
