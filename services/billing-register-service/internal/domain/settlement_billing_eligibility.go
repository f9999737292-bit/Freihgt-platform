package domain

import (
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const (
	BillingBlockNotApproved      = "NOT_APPROVED"
	BillingBlockOpenDispute      = "OPEN_DISPUTE"
	BillingBlockAlreadyIncluded  = "ALREADY_INCLUDED"
	BillingBlockCurrencyMismatch = "CURRENCY_MISMATCH"
	BillingBlockWrongRelationship = "WRONG_RELATIONSHIP"
)

type SettlementBillingEligibility struct {
	Eligible          bool
	BlockReason       string
	EligibleStatuses  []string
}

var settlementBillingEligibleStatuses = map[string]struct{}{
	SettlementStatusApproved:        {},
	SettlementStatusDocumentsReady:  {},
	SettlementStatusReadyForPayment: {},
}

func SettlementEligibleStatusesForBilling() []string {
	return []string{
		SettlementStatusApproved,
		SettlementStatusDocumentsReady,
		SettlementStatusReadyForPayment,
	}
}

func EvaluateSettlementBillingEligibility(settlement *FreightSettlement, hasOpenDispute bool, registerCurrency string) SettlementBillingEligibility {
	if settlement.BillingRegisterID != nil {
		return SettlementBillingEligibility{Eligible: false, BlockReason: BillingBlockAlreadyIncluded}
	}
	if _, ok := settlementBillingEligibleStatuses[settlement.Status]; !ok {
		return SettlementBillingEligibility{Eligible: false, BlockReason: BillingBlockNotApproved}
	}
	if hasOpenDispute {
		return SettlementBillingEligibility{Eligible: false, BlockReason: BillingBlockOpenDispute}
	}
	if registerCurrency != "" && NormalizeCurrencyCode(registerCurrency) != NormalizeCurrencyCode(settlement.CurrencyCode) {
		return SettlementBillingEligibility{Eligible: false, BlockReason: BillingBlockCurrencyMismatch}
	}
	return SettlementBillingEligibility{Eligible: true}
}

func ValidateSettlementForBillingInclusion(settlement *FreightSettlement, hasOpenDispute bool, register *BillingRegister) error {
	eligibility := EvaluateSettlementBillingEligibility(settlement, hasOpenDispute, register.CurrencyCode)
	if !eligibility.Eligible {
		return apperrors.Validation("settlement is not eligible for billing register inclusion", map[string]any{
			"field": "settlement_id", "reason": eligibility.BlockReason, "status": settlement.Status,
		})
	}
	if settlement.BuyerCompanyID != register.CustomerCompanyID || settlement.CarrierCompanyID != register.ContractorCompanyID {
		return apperrors.Forbidden("settlement buyer/carrier does not match billing register")
	}
	return nil
}

func ValidateBillingRegisterAccess(reg *BillingRegister, actorCompanyID uuid.UUID, actorKind string) error {
	switch actorKind {
	case SettlementActorBuyer:
		if reg.CustomerCompanyID != actorCompanyID {
			return apperrors.Forbidden("buyer cannot access another buyer's billing register")
		}
	case SettlementActorCarrier:
		if reg.ContractorCompanyID != actorCompanyID {
			return apperrors.Forbidden("carrier cannot access another carrier's billing register")
		}
	default:
		return apperrors.Validation("actor must be CARRIER or BUYER", map[string]any{"field": "actor"})
	}
	return nil
}
