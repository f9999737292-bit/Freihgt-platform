package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type MembershipLookup interface {
	ListUserCompanyMemberships(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.UserCompanyMembership, error)
	ListUserGlobalRoleCodes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)
}

type ActorResolver struct {
	memberships MembershipLookup
}

func NewActorResolver(memberships MembershipLookup) *ActorResolver {
	return &ActorResolver{memberships: memberships}
}

func (a *ActorResolver) FromRequest(r *http.Request) (domain.ActorInput, error) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return domain.ActorInput{}, err
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		return domain.ActorInput{}, err
	}
	companyID, actorKind, err := resolveTrustedCompanyContext(r)
	if err != nil {
		return domain.ActorInput{}, err
	}
	memberships, err := a.memberships.ListUserCompanyMemberships(r.Context(), tenantID, userID)
	if err != nil {
		return domain.ActorInput{}, err
	}
	globalRoles, err := a.memberships.ListUserGlobalRoleCodes(r.Context(), tenantID, userID)
	if err != nil {
		return domain.ActorInput{}, err
	}
	return domain.ResolveTrustedActor(tenantID, userID, companyID, actorKind, memberships, domain.HasPlatformAdminRole(globalRoles))
}

func CorrelationID(r *http.Request) *string {
	raw := strings.TrimSpace(r.Header.Get(domain.HeaderRequestID))
	if raw == "" {
		return nil
	}
	return &raw
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
	if actorKind != domain.ActorKindBuyer && actorKind != domain.ActorKindCarrier {
		return uuid.Nil, "", apperrors.Forbidden("verified actor context is required")
	}
	return companyID, actorKind, nil
}
