package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const (
	HeaderTenantID  = "X-Tenant-ID"
	HeaderUserID    = "X-User-ID"
	HeaderCompanyID = "X-Company-ID"
	HeaderActorKind = "X-Actor-Kind"

	PaymentActorBuyer   = "BUYER"
	PaymentActorCarrier = "CARRIER"
)

type PaymentActorInput struct {
	TenantID       uuid.UUID
	ActorCompanyID uuid.UUID
	ActorKind      string
	ActorUserID    uuid.UUID
}

type UserCompanyMembership struct {
	CompanyID   uuid.UUID
	CompanyType string
	RoleCodes   []string
}

type PaymentObligation struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	ObligationNumber   string
	PayerCompanyID     uuid.UUID
	PayeeCompanyID     uuid.UUID
	SourceType         string
	SourceID           uuid.UUID
	CurrencyCode       string
	OriginalAmount     decimal.Decimal
	PaidAmount         decimal.Decimal
	OutstandingAmount  decimal.Decimal
	DueDate            *time.Time
	Status             string
	BlockedReason      *string
	Version            int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Payment struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	PaymentNumber     string
	PayerCompanyID    uuid.UUID
	PayeeCompanyID    uuid.UUID
	Amount            decimal.Decimal
	CurrencyCode      string
	PaymentDate       time.Time
	ValueDate         *time.Time
	Reference         *string
	ExternalReference *string
	Source            string
	ExternalID        *string
	Status            string
	AllocatedAmount   decimal.Decimal
	UnallocatedAmount decimal.Decimal
	CreatedBy         uuid.UUID
	ReconciledAt      *time.Time
	ReconciledBy      *uuid.UUID
	VoidedAt          *time.Time
	VoidedBy          *uuid.UUID
	VoidReason        *string
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PaymentAllocation struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	PaymentID       uuid.UUID
	ObligationID    uuid.UUID
	AllocatedAmount decimal.Decimal
	CurrencyCode    string
	CreatedBy       uuid.UUID
	CreatedAt       time.Time
	VoidedAt        *time.Time
}

type CreateManualPaymentInput struct {
	TenantID          uuid.UUID
	Amount            decimal.Decimal
	CurrencyCode      string
	PaymentDate       time.Time
	PayerCompanyID    uuid.UUID
	PayeeCompanyID    uuid.UUID
	Reference         *string
	ExternalReference *string
	ExternalID        *string
	Source            string
	CreatedBy         uuid.UUID
}

type CreateAllocationInput struct {
	TenantID         uuid.UUID
	PaymentID        uuid.UUID
	ObligationID     uuid.UUID
	AllocatedAmount  decimal.Decimal
	CreatedBy        uuid.UUID
	ActorCompanyID   uuid.UUID
	ActorKind        string
}

func ValidatePaymentActor(actor PaymentActorInput) error {
	if actor.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if actor.ActorUserID == uuid.Nil {
		return apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	if actor.ActorCompanyID == uuid.Nil {
		return apperrors.Forbidden("verified company context is required")
	}
	if actor.ActorKind != PaymentActorBuyer && actor.ActorKind != PaymentActorCarrier {
		return apperrors.Forbidden("verified actor context is required")
	}
	return nil
}

func HasPlatformAdminRole(roleCodes []string) bool {
	for _, code := range roleCodes {
		if strings.EqualFold(code, "PLATFORM_ADMIN") {
			return true
		}
	}
	return false
}

func ResolveTrustedPaymentActor(
	tenantID, userID, companyID uuid.UUID,
	actorKind string,
	memberships []UserCompanyMembership,
	platformAdmin bool,
) (PaymentActorInput, error) {
	_ = platformAdmin
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return PaymentActorInput{}, apperrors.Unauthorized("authenticated identity is required")
	}
	if companyID == uuid.Nil {
		return PaymentActorInput{}, apperrors.Forbidden("verified company context is required")
	}
	kind := strings.ToUpper(strings.TrimSpace(actorKind))
	if kind != PaymentActorBuyer && kind != PaymentActorCarrier {
		return PaymentActorInput{}, apperrors.Forbidden("verified actor context is required")
	}
	found := false
	for _, m := range memberships {
		if m.CompanyID == companyID {
			found = true
			break
		}
	}
	if !found {
		return PaymentActorInput{}, apperrors.Forbidden("company membership is required")
	}
	return PaymentActorInput{
		TenantID: tenantID, ActorCompanyID: companyID, ActorKind: kind, ActorUserID: userID,
	}, nil
}

func ValidatePaymentAccess(payer, payee, actorCompany uuid.UUID, actorKind string) error {
	switch actorKind {
	case PaymentActorBuyer:
		if payer != actorCompany {
			return apperrors.Forbidden("buyer cannot access another company's payment obligations")
		}
	case PaymentActorCarrier:
		if payee != actorCompany {
			return apperrors.Forbidden("carrier cannot access another company's payment obligations")
		}
	default:
		return apperrors.Forbidden("verified actor context is required")
	}
	return nil
}

func ValidateAllocationParties(
	paymentPayer, paymentPayee uuid.UUID,
	obligationPayer, obligationPayee uuid.UUID,
	paymentCurrency, obligationCurrency string,
	allocationAmount, paymentUnallocated, obligationOutstanding decimal.Decimal,
) error {
	if paymentPayer != obligationPayer || paymentPayee != obligationPayee {
		return apperrors.Conflict("payment and obligation parties must match", nil)
	}
	if paymentCurrency != obligationCurrency {
		return apperrors.Conflict("payment and obligation currency must match", map[string]any{
			"payment_currency": paymentCurrency, "obligation_currency": obligationCurrency,
		})
	}
	if allocationAmount.LessThanOrEqual(decimal.Zero) {
		return apperrors.Validation("allocated_amount must be greater than zero", map[string]any{"field": "allocated_amount"})
	}
	if allocationAmount.GreaterThan(paymentUnallocated) {
		return apperrors.Conflict("allocation exceeds payment unallocated amount", map[string]any{
			"unallocated_amount": paymentUnallocated.StringFixed(MoneyScale),
		})
	}
	if allocationAmount.GreaterThan(obligationOutstanding) {
		return apperrors.Conflict("allocation exceeds obligation outstanding amount", map[string]any{
			"outstanding_amount": obligationOutstanding.StringFixed(MoneyScale),
		})
	}
	return nil
}

func EnforceOptionalBodyTenant(trusted uuid.UUID, bodyTenantRaw string) error {
	raw := strings.TrimSpace(bodyTenantRaw)
	if raw == "" {
		return nil
	}
	bodyTenant, err := ParseUUID(raw, "tenant_id")
	if err != nil {
		return err
	}
	if bodyTenant != trusted {
		return apperrors.Forbidden("tenant_id does not match authenticated tenant")
	}
	return nil
}
