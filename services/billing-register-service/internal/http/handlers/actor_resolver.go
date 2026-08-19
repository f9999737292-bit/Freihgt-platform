package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type MembershipLookup interface {
	ListUserCompanyMemberships(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.UserCompanyMembership, error)
	ListUserGlobalRoleCodes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)
}

type SettlementActorResolver struct {
	memberships MembershipLookup
}

func NewSettlementActorResolver(memberships MembershipLookup) *SettlementActorResolver {
	return &SettlementActorResolver{memberships: memberships}
}

func (a *SettlementActorResolver) FromRequest(r *http.Request) (domain.SettlementActorInput, error) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	companyID, actorKind, err := resolveTrustedCompanyContext(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	if err := rejectSpoofedQueryIdentity(r, companyID, actorKind); err != nil {
		return domain.SettlementActorInput{}, err
	}

	memberships, err := a.memberships.ListUserCompanyMemberships(r.Context(), tenantID, userID)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	globalRoles, err := a.memberships.ListUserGlobalRoleCodes(r.Context(), tenantID, userID)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}

	return domain.ResolveTrustedSettlementActor(
		tenantID, userID, companyID, actorKind, memberships, domain.HasPlatformAdminRole(globalRoles),
	)
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
	if actorKind != domain.SettlementActorBuyer && actorKind != domain.SettlementActorCarrier {
		return uuid.Nil, "", apperrors.Forbidden("verified actor context is required")
	}
	return companyID, actorKind, nil
}

func rejectSpoofedQueryIdentity(r *http.Request, trustedCompany uuid.UUID, trustedActor string) error {
	if raw := strings.TrimSpace(r.URL.Query().Get("company_id")); raw != "" {
		queryCompany, err := domain.ParseUUID(raw, "company_id")
		if err != nil {
			return err
		}
		if queryCompany != trustedCompany {
			return apperrors.Forbidden("company_id does not match verified company context")
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("actor")); raw != "" {
		if strings.ToUpper(raw) != trustedActor {
			return apperrors.Forbidden("actor does not match verified actor context")
		}
	}
	return nil
}

// parseSettlementActorFromQuery documents the pre-v1.8.2 insecure path used only in reproduction tests.
func parseSettlementActorFromQuery(r *http.Request) (uuid.UUID, string, error) {
	actorKind := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("actor")))
	companyRaw := strings.TrimSpace(r.URL.Query().Get("company_id"))
	if companyRaw == "" {
		return uuid.Nil, "", apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	companyID, err := domain.ParseUUID(companyRaw, "company_id")
	if err != nil {
		return uuid.Nil, "", err
	}
	switch actorKind {
	case domain.SettlementActorCarrier, domain.SettlementActorBuyer:
		return companyID, actorKind, nil
	default:
		return uuid.Nil, "", apperrors.Validation("actor must be CARRIER or BUYER", map[string]any{"field": "actor"})
	}
}

func legacySettlementActorInput(r *http.Request) (domain.SettlementActorInput, error) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	companyID, actorKind, err := parseSettlementActorFromQuery(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	return domain.SettlementActorInput{
		TenantID: tenantID, ActorCompanyID: companyID, ActorKind: actorKind, ActorUserID: userID,
	}, nil
}
