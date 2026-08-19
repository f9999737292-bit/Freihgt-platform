package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

type MembershipLookup interface {
	ListUserCompanyMemberships(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.UserCompanyMembership, error)
	ListUserGlobalRoleCodes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)
}

type PaymentActorResolver struct {
	memberships MembershipLookup
}

func NewPaymentActorResolver(memberships MembershipLookup) *PaymentActorResolver {
	return &PaymentActorResolver{memberships: memberships}
}

func (a *PaymentActorResolver) FromRequest(r *http.Request) (domain.PaymentActorInput, error) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return domain.PaymentActorInput{}, err
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		return domain.PaymentActorInput{}, err
	}
	companyID, actorKind, err := resolveTrustedCompanyContext(r)
	if err != nil {
		return domain.PaymentActorInput{}, err
	}
	memberships, err := a.memberships.ListUserCompanyMemberships(r.Context(), tenantID, userID)
	if err != nil {
		return domain.PaymentActorInput{}, err
	}
	globalRoles, err := a.memberships.ListUserGlobalRoleCodes(r.Context(), tenantID, userID)
	if err != nil {
		return domain.PaymentActorInput{}, err
	}
	return domain.ResolveTrustedPaymentActor(tenantID, userID, companyID, actorKind, memberships, domain.HasPlatformAdminRole(globalRoles))
}

func resolveVerifiedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get(domain.HeaderTenantID))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("authenticated tenant is required")
	}
	return domain.ParseUUID(raw, "tenant_id")
}

func resolveVerifiedUser(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get(domain.HeaderUserID))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("authenticated user is required")
	}
	return domain.ParseUUID(raw, "user_id")
}

func resolveTrustedCompanyContext(r *http.Request) (uuid.UUID, string, error) {
	companyRaw := strings.TrimSpace(r.Header.Get(domain.HeaderCompanyID))
	if companyRaw == "" {
		return uuid.Nil, "", apperrors.Forbidden("verified company context is required")
	}
	companyID, err := domain.ParseUUID(companyRaw, "company_id")
	if err != nil {
		return uuid.Nil, "", err
	}
	actorKind := strings.ToUpper(strings.TrimSpace(r.Header.Get(domain.HeaderActorKind)))
	if actorKind != domain.PaymentActorBuyer && actorKind != domain.PaymentActorCarrier {
		return uuid.Nil, "", apperrors.Forbidden("verified actor context is required")
	}
	return companyID, actorKind, nil
}
