package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const (
	SettlementStatusDraft            = "DRAFT"
	SettlementStatusUnderReview      = "UNDER_REVIEW"
	SettlementStatusDisputed         = "DISPUTED"
	SettlementStatusApproved         = "APPROVED"
	SettlementStatusDocumentsReady   = "DOCUMENTS_READY"
	SettlementStatusReadyForPayment  = "READY_FOR_PAYMENT"
	SettlementStatusCancelled        = "CANCELLED"

	SettlementActorCarrier = "CARRIER"
	SettlementActorBuyer   = "BUYER"

	ShipmentStatusDelivered          = "DELIVERED"
)

type FreightSettlement struct {
	ID                       uuid.UUID
	TenantID                 uuid.UUID
	ShipmentID               uuid.UUID
	TransportOrderID         uuid.UUID
	BuyerCompanyID           uuid.UUID
	CarrierCompanyID         uuid.UUID
	AwardLinkID              *uuid.UUID
	SettlementNumber         string
	BaseFreightAmount        float64
	CurrencyCode             string
	VATRate                  *float64
	ApprovedAccessorialTotal float64
	TotalWithoutVAT          float64
	VATAmount                float64
	TotalWithVAT             float64
	Status                   string
	ServiceAcceptedAt        *time.Time
	ServiceAcceptedBy        *uuid.UUID
	BillingRegisterID        *uuid.UUID
	BillingRegisterItemID    *uuid.UUID
	IdempotencyKey           *string
	Version                  int
	CreatedAt                time.Time
	CreatedBy                *uuid.UUID
	UpdatedAt                time.Time
}

type CreateFreightSettlementInput struct {
	TenantID         uuid.UUID
	ShipmentID       uuid.UUID
	ActorCompanyID   uuid.UUID
	ActorKind        string
	ActorUserID      uuid.UUID
	IdempotencyKey   string
	SettlementNumber string
}

type ListFreightSettlementsFilter struct {
	TenantID         uuid.UUID
	BuyerCompanyID   *uuid.UUID
	CarrierCompanyID *uuid.UUID
	Status           *string
	Limit            int
	Offset           int
}

type SettlementActorInput struct {
	TenantID       uuid.UUID
	ActorCompanyID uuid.UUID
	ActorKind      string
	ActorUserID    uuid.UUID
}

func ValidateSettlementActor(in SettlementActorInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.ActorCompanyID == uuid.Nil {
		return apperrors.Validation("actor_company_id is required", map[string]any{"field": "actor_company_id"})
	}
	if in.ActorUserID == uuid.Nil {
		return apperrors.Validation("actor_user_id is required", map[string]any{"field": "actor_user_id"})
	}
	switch strings.ToUpper(strings.TrimSpace(in.ActorKind)) {
	case SettlementActorCarrier, SettlementActorBuyer:
		return nil
	default:
		return apperrors.Validation("actor must be CARRIER or BUYER", map[string]any{"field": "actor"})
	}
}

func ValidateSettlementAccess(settlement *FreightSettlement, actorCompanyID uuid.UUID, actorKind string) error {
	switch strings.ToUpper(strings.TrimSpace(actorKind)) {
	case SettlementActorCarrier:
		if settlement.CarrierCompanyID != actorCompanyID {
			return apperrors.Forbidden("carrier cannot access another carrier's settlement")
		}
	case SettlementActorBuyer:
		if settlement.BuyerCompanyID != actorCompanyID {
			return apperrors.Forbidden("buyer cannot access another buyer's settlement")
		}
	default:
		return apperrors.Validation("actor must be CARRIER or BUYER", map[string]any{"field": "actor"})
	}
	return nil
}

func ValidateShipmentEligibleForSettlement(status string, hasPOD bool) error {
	switch status {
	case ShipmentStatusDelivered, ShipmentStatusReadyForBilling, ShipmentStatusDocumentsCompleted:
		if !hasPOD {
			return apperrors.Validation("shipment must have POD evidence before settlement", map[string]any{"field": "shipment_id"})
		}
		return nil
	default:
		return apperrors.Validation("shipment is not eligible for settlement", map[string]any{
			"field": "shipment_id", "status": status,
		})
	}
}

func ValidateSettlementTransition(from, to string) error {
	allowed := map[string]map[string]struct{}{
		SettlementStatusDraft: {
			SettlementStatusUnderReview: {},
			SettlementStatusCancelled:   {},
		},
		SettlementStatusUnderReview: {
			SettlementStatusDisputed: {},
			SettlementStatusApproved: {},
			SettlementStatusDraft:    {},
		},
		SettlementStatusDisputed: {
			SettlementStatusUnderReview: {},
			SettlementStatusApproved:  {},
		},
		SettlementStatusApproved: {
			SettlementStatusDocumentsReady: {},
			SettlementStatusDisputed:       {},
		},
		SettlementStatusDocumentsReady: {
			SettlementStatusReadyForPayment: {},
		},
	}
	next, ok := allowed[from]
	if !ok {
		return apperrors.Validation("settlement status transition not allowed", map[string]any{"from": from, "to": to})
	}
	if _, ok := next[to]; !ok {
		return apperrors.Validation("settlement status transition not allowed", map[string]any{"from": from, "to": to})
	}
	return nil
}

func CalculateSettlementTotals(baseFreight, approvedAccessorial float64, vatRate *float64) (withoutVAT, vat, withVAT float64) {
	withoutVAT = round2(baseFreight + approvedAccessorial)
	vat = 0.0
	if vatRate != nil {
		vat = round2(withoutVAT * (*vatRate) / 100)
	}
	withVAT = round2(withoutVAT + vat)
	return withoutVAT, vat, withVAT
}
