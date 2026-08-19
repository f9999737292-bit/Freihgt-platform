package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

// EnforceOptionalBodyTenant ensures a client-supplied tenant_id cannot override trusted gateway tenant.
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

// ValidateRegisterBuyerMutation ensures buyer actor owns the register customer company.
func ValidateRegisterBuyerMutation(reg *BillingRegister, actor SettlementActorInput) error {
	if err := ValidateSettlementActor(actor); err != nil {
		return err
	}
	if actor.ActorKind != SettlementActorBuyer {
		return apperrors.Forbidden("only buyer can mutate billing register")
	}
	return ValidateBillingRegisterAccess(reg, actor.ActorCompanyID, actor.ActorKind)
}

// ResolveApprovedBy returns verified user id; rejects spoofed approved_by when provided.
func ResolveApprovedBy(trustedUser uuid.UUID, bodyApprovedByRaw string) (uuid.UUID, error) {
	raw := strings.TrimSpace(bodyApprovedByRaw)
	if raw == "" {
		return trustedUser, nil
	}
	bodyApprovedBy, err := ParseUUID(raw, "approved_by")
	if err != nil {
		return uuid.Nil, err
	}
	if bodyApprovedBy != trustedUser {
		return uuid.Nil, apperrors.Forbidden("approved_by does not match authenticated user")
	}
	return trustedUser, nil
}
