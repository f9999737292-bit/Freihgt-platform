package companycontext

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

// CarrierReadSelection identifies which client-supplied query value is
// company context. It is never treated as authority until membership matches.
type CarrierReadSelection int

const (
	// SelectCarrierCompany constrains fleet and carrier transport-order lists
	// with carrier_company_id.
	SelectCarrierCompany CarrierReadSelection = iota
	// SelectExecutionCompany constrains execution detail with company_id and
	// a server-derived CARRIER actor.
	SelectExecutionCompany
)

// RequireCarrierRead authorizes a company-scoped customer read from the
// selected membership only, then proxies. next must be the downstream proxy.
// A later tenant-wide /auth/me role check would let a role on another company
// authorize this one. Client X-Company-ID and X-Actor-Kind are replaced.
func (e *Enforcer) RequireCarrierRead(selection CarrierReadSelection, next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		StripUntrustedCompanyHeaders(r.Header)
		if !e.authEnabled {
			next.ServeHTTP(w, r)
			return
		}

		reqCtx, err := e.buildRequestContext(r)
		if err != nil {
			respond.Error(w, err)
			return
		}
		if reqCtx.UserID == "" {
			respond.Error(w, apperrors.Unauthorized("verified user context is required"))
			return
		}

		selectionKey := carrierCompanyQueryKey
		if selection == SelectExecutionCompany {
			selectionKey = executionCompanyQueryKey
		}
		query := r.URL.Query()
		requestedRaw := strings.TrimSpace(query.Get(selectionKey))
		if requestedRaw == "" {
			respond.Error(w, apperrors.Validation(selectionKey+" is required", map[string]any{"field": selectionKey}))
			return
		}
		requestedCompanyID, err := uuid.Parse(requestedRaw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid "+selectionKey, map[string]any{"field": selectionKey}))
			return
		}

		memberships, err := e.client.ListUserCompanies(r.Context(), reqCtx, reqCtx.UserID)
		if err != nil {
			respond.Error(w, membershipLookupError(err))
			return
		}

		var matched *UserCompany
		for i := range memberships {
			parsed, parseErr := uuid.Parse(memberships[i].CompanyID)
			if parseErr != nil {
				continue
			}
			if parsed == requestedCompanyID {
				matched = &memberships[i]
				break
			}
		}
		if matched == nil {
			respond.Error(w, apperrors.Forbidden("company does not match authenticated membership"))
			return
		}
		actorKind, err := e.actorForSelection(r, reqCtx, selection, matched)
		if err != nil {
			respond.Error(w, err)
			return
		}

		canonical := requestedCompanyID.String()
		if err := rejectForeignCompanyQuery(query, canonical); err != nil {
			respond.Error(w, err)
			return
		}
		if selection == SelectExecutionCompany {
			if err := rejectForeignActorQuery(query, actorKind); err != nil {
				respond.Error(w, err)
				return
			}
			query.Set(executionActorQueryKey, actorKind)
		}
		if len(query[carrierCompanyQueryKey]) > 0 {
			query.Set(carrierCompanyQueryKey, canonical)
		}
		if len(query[executionCompanyQueryKey]) > 0 {
			query.Set(executionCompanyQueryKey, canonical)
		}
		query.Set(selectionKey, canonical)
		r.URL.RawQuery = query.Encode()
		r.Header.Set(HeaderCompanyID, canonical)
		r.Header.Set(HeaderActorKind, actorKind)
		next.ServeHTTP(w, r)
	})
}

const (
	carrierCompanyQueryKey   = "carrier_company_id"
	executionCompanyQueryKey = "company_id"
	executionActorQueryKey   = "actor"
)

func rejectForeignCompanyQuery(query url.Values, canonical string) error {
	verified, err := uuid.Parse(canonical)
	if err != nil {
		return apperrors.Forbidden("company does not match authenticated membership")
	}
	for _, key := range []string{carrierCompanyQueryKey, executionCompanyQueryKey} {
		for _, raw := range query[key] {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			parsed, parseErr := uuid.Parse(raw)
			if parseErr != nil {
				return apperrors.Validation("invalid "+key, map[string]any{"field": key})
			}
			if parsed != verified {
				return apperrors.Forbidden("company does not match authenticated membership")
			}
		}
	}
	return nil
}

func (e *Enforcer) actorForSelection(r *http.Request, reqCtx routeauth.RequestContext, selection CarrierReadSelection, matched *UserCompany) (string, error) {
	switch selection {
	case SelectCarrierCompany:
		if !MembershipAllowsCarrierRead(matched.CompanyType, matched.RoleCodes) {
			return "", apperrors.Forbidden("company is not authorized for carrier access")
		}
		return ActorCarrier, nil
	case SelectExecutionCompany:
		actor, err := ExecutionActorFromMembership(matched.CompanyType, matched.RoleCodes)
		if err == nil {
			return actor, nil
		}
		// Tenant-global PLATFORM_ADMIN is not a role attached to another company.
		// It keeps execution read for a membership that already derives BUYER.
		derived, deriveErr := DeriveActorKind(matched.CompanyType, matched.RoleCodes)
		if deriveErr != nil || derived != ActorBuyer {
			return "", err
		}
		tenantRoles, tenantErr := e.client.ListUserTenantRoles(r.Context(), reqCtx, reqCtx.UserID)
		if tenantErr != nil {
			return "", membershipLookupError(tenantErr)
		}
		if hasTenantPlatformAdmin(tenantRoles) {
			return ActorBuyer, nil
		}
		return "", err
	default:
		return "", apperrors.Forbidden("company is not authorized for carrier access")
	}
}

func rejectForeignActorQuery(query url.Values, derived string) error {
	for _, raw := range query[executionActorQueryKey] {
		actor := strings.ToUpper(strings.TrimSpace(raw))
		if actor == "" {
			continue
		}
		if actor != derived {
			return apperrors.Forbidden("actor does not match verified company membership")
		}
	}
	return nil
}

func membershipLookupError(err error) error {
	switch {
	case errors.Is(err, routeauth.ErrIdentityUnauthorized):
		return apperrors.Unauthorized("invalid or expired token")
	case errors.Is(err, routeauth.ErrIdentityForbidden):
		return apperrors.Forbidden("insufficient permission")
	default:
		return apperrors.AuthDependencyUnavailable("authentication service is temporarily unavailable")
	}
}
