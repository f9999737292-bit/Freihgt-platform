package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type InternalSettlementRead struct {
	SettlementID        uuid.UUID
	TenantID            uuid.UUID
	TransportOrderID    uuid.UUID
	ShipmentID          uuid.UUID
	BuyerCompanyID      uuid.UUID
	CarrierCompanyID    uuid.UUID
	Status              string
	OpenDisputeCount    int
	Version             int
	BillingLinkRevision int64
	BillingLinkState    string
	CurrencyCode        string
	BaseFreightAmount               string
	AccrualAmountExVAT              string
	TotalWithoutVAT                 string
	ProposedAccessorialTotalExVAT   string
	ProposedAccessorialSourceStatus string
	RateSnapshotID                  *uuid.UUID
	UpdatedAt           time.Time
}

type InternalBillingLinkRead struct {
	SettlementID        uuid.UUID
	TenantID            uuid.UUID
	TransportOrderID    uuid.UUID
	BillingLinkRevision int64
	BillingLinkState    string
	AmountExVAT         *string
	BillingRegisterID   *uuid.UUID
	CurrencyCode        string
	TaxBasis            string
}

type InternalRegisterPayableRead struct {
	RegisterID   uuid.UUID
	TenantID     uuid.UUID
	Status       string
	Version      int
	CurrencyCode string
	TotalWithVAT string
	TaxBasis     string
	UpdatedAt    time.Time
}

func (r *FreightSettlementRepository) GetInternalByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*InternalSettlementRead, error) {
	const query = `
		SELECT fs.id, fs.tenant_id, fs.transport_order_id, fs.shipment_id,
			fs.buyer_company_id, fs.carrier_company_id, fs.status, fs.version,
			fs.billing_link_revision,
			CASE WHEN fs.billing_register_id IS NULL THEN 'UNLINKED' ELSE 'LINKED' END,
			fs.currency_code,
			fs.base_freight_amount::text,
			fs.total_without_vat::text,
			fs.rate_snapshot_id,
			fs.updated_at,
			(SELECT COUNT(*) FROM billing.settlement_disputes d
			 WHERE d.settlement_id = fs.id AND d.tenant_id = fs.tenant_id AND d.status = $3),
			COALESCE((
				SELECT SUM(a.amount)::text FROM billing.settlement_accessorials a
				WHERE a.settlement_id = fs.id AND a.tenant_id = fs.tenant_id AND a.status = $4
			), '0')::text,
			COALESCE((
				SELECT SUM(a.amount)::text FROM billing.settlement_accessorials a
				WHERE a.settlement_id = fs.id AND a.tenant_id = fs.tenant_id AND a.status = $5
			), '0')::text
		FROM billing.freight_settlements fs
		WHERE fs.transport_order_id = $1 AND fs.tenant_id = $2 AND fs.deleted_at IS NULL
		ORDER BY fs.created_at DESC
		LIMIT 1`
	row := r.pool.QueryRow(ctx, query, transportOrderID, tenantID, domain.DisputeStatusOpen, domain.AccessorialStatusApproved, domain.AccessorialStatusProposed)
	read, err := scanInternalSettlementRead(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("freight settlement not found")
	}
	return read, err
}

func (r *FreightSettlementRepository) GetInternalBillingLink(ctx context.Context, tenantID, settlementID uuid.UUID) (*InternalBillingLinkRead, error) {
	const query = `
		SELECT id, tenant_id, transport_order_id, billing_link_revision,
			CASE WHEN billing_register_id IS NULL THEN 'UNLINKED' ELSE 'LINKED' END,
			total_without_vat::text, billing_register_id, currency_code
		FROM billing.freight_settlements
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var read InternalBillingLinkRead
	var amountText string
	var registerID *uuid.UUID
	err := r.pool.QueryRow(ctx, query, settlementID, tenantID).Scan(
		&read.SettlementID, &read.TenantID, &read.TransportOrderID, &read.BillingLinkRevision,
		&read.BillingLinkState, &amountText, &registerID, &read.CurrencyCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("freight settlement not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	read.TaxBasis = domain.TaxBasisExVAT
	if read.BillingLinkState == domain.BillingLinkStateLinked {
		normalized, normErr := normalizeDecimalMoneyString(amountText)
		if normErr != nil {
			return nil, normErr
		}
		read.AmountExVAT = &normalized
		read.BillingRegisterID = registerID
	}
	return &read, nil
}

func (r *BillingRegisterRepository) GetInternalPayable(ctx context.Context, tenantID, registerID uuid.UUID) (*InternalRegisterPayableRead, error) {
	const query = `
		SELECT id, tenant_id, status, version, currency_code, total_with_vat::text, updated_at
		FROM billing.billing_registers
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var read InternalRegisterPayableRead
	var totalText string
	err := r.pool.QueryRow(ctx, query, registerID, tenantID).Scan(
		&read.RegisterID, &read.TenantID, &read.Status, &read.Version, &read.CurrencyCode, &totalText, &read.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("billing register not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	normalized, normErr := normalizeDecimalMoneyString(totalText)
	if normErr != nil {
		return nil, normErr
	}
	read.TotalWithVAT = normalized
	read.TaxBasis = domain.TaxBasisWithVAT
	return &read, nil
}

type internalSettlementRow interface {
	Scan(dest ...any) error
}

func scanInternalSettlementRead(row internalSettlementRow) (*InternalSettlementRead, error) {
	var read InternalSettlementRead
	var baseText, totalText, approvedSumText, proposedSumText string
	err := row.Scan(
		&read.SettlementID, &read.TenantID, &read.TransportOrderID, &read.ShipmentID,
		&read.BuyerCompanyID, &read.CarrierCompanyID, &read.Status, &read.Version,
		&read.BillingLinkRevision, &read.BillingLinkState, &read.CurrencyCode,
		&baseText, &totalText, &read.RateSnapshotID, &read.UpdatedAt,
		&read.OpenDisputeCount, &approvedSumText, &proposedSumText,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	base, err := normalizeDecimalMoneyString(baseText)
	if err != nil {
		return nil, err
	}
	total, err := normalizeDecimalMoneyString(totalText)
	if err != nil {
		return nil, err
	}
	approvedSum, err := normalizeDecimalMoneyString(approvedSumText)
	if err != nil {
		return nil, err
	}
	baseDec, _ := decimal.NewFromString(base)
	approvedDec, _ := decimal.NewFromString(approvedSum)
	accrual := baseDec.Add(approvedDec).StringFixed(domain.MoneyScale)
	read.BaseFreightAmount = base
	read.TotalWithoutVAT = total
	read.AccrualAmountExVAT = accrual
	proposedSum, err := normalizeDecimalMoneyString(proposedSumText)
	if err != nil {
		return nil, err
	}
	read.ProposedAccessorialTotalExVAT = proposedSum
	read.ProposedAccessorialSourceStatus = "KNOWN"
	return &read, nil
}

func normalizeDecimalMoneyString(raw string) (string, error) {
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return "", apperrors.Internal("invalid money decimal", err)
	}
	return d.StringFixed(domain.MoneyScale), nil
}

func querySettlementSnapshotMoneyTx(
	ctx context.Context,
	tx pgx.Tx,
	settlementID, tenantID uuid.UUID,
) (accrualExVAT, totalWithoutVAT string, err error) {
	// Accrual principal is settlement.base_freight_amount, an immutable copy of
	// transport_order_rate_snapshots.total_amount at SNAPSHOT_V1 settlement create.
	// Accrual = principal + exact NUMERIC SUM(APPROVED accessorials).
	const query = `
		SELECT fs.base_freight_amount::text, fs.total_without_vat::text,
			COALESCE((
				SELECT SUM(a.amount)::text FROM billing.settlement_accessorials a
				WHERE a.settlement_id = fs.id AND a.tenant_id = fs.tenant_id AND a.status = $3
			), '0')::text
		FROM billing.freight_settlements fs
		WHERE fs.id = $1 AND fs.tenant_id = $2 AND fs.deleted_at IS NULL`
	var baseText, totalText, approvedSumText string
	if err := tx.QueryRow(ctx, query, settlementID, tenantID, domain.AccessorialStatusApproved).Scan(
		&baseText, &totalText, &approvedSumText,
	); err != nil {
		return "", "", mapDBError(err)
	}
	base, err := normalizeDecimalMoneyString(baseText)
	if err != nil {
		return "", "", err
	}
	total, err := normalizeDecimalMoneyString(totalText)
	if err != nil {
		return "", "", err
	}
	approvedSum, err := normalizeDecimalMoneyString(approvedSumText)
	if err != nil {
		return "", "", err
	}
	baseDec, _ := decimal.NewFromString(base)
	approvedDec, _ := decimal.NewFromString(approvedSum)
	return baseDec.Add(approvedDec).StringFixed(domain.MoneyScale), total, nil
}

func queryRegisterPayableAmountTx(
	ctx context.Context,
	tx pgx.Tx,
	registerID, tenantID uuid.UUID,
) (string, error) {
	const query = `
		SELECT total_with_vat::text
		FROM billing.billing_registers
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var raw string
	if err := tx.QueryRow(ctx, query, registerID, tenantID).Scan(&raw); err != nil {
		return "", mapDBError(err)
	}
	return normalizeDecimalMoneyString(raw)
}

func querySettlementTotalWithoutVATTx(
	ctx context.Context,
	tx pgx.Tx,
	settlementID, tenantID uuid.UUID,
) (string, error) {
	const query = `
		SELECT total_without_vat::text
		FROM billing.freight_settlements
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var raw string
	if err := tx.QueryRow(ctx, query, settlementID, tenantID).Scan(&raw); err != nil {
		return "", mapDBError(err)
	}
	return normalizeDecimalMoneyString(raw)
}
