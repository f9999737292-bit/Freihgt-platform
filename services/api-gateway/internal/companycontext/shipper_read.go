package companycontext

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

const shipperCompanyQueryKey = "shipper_company_id"

// RequireShipperRead authorizes a customer shipment read from the selected
// shipper membership only, then proxies. shipper_company_id is selection
// input. A tenant-global PLATFORM_ADMIN role and a role on another company
// do not authorize the selected company. Client X-Company-ID and
// X-Actor-Kind are replaced with the verified company and server-derived BUYER.
func (e *Enforcer) RequireShipperRead(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		query := r.URL.Query()
		requestedCompanyID, err := parseShipperCompanyQuery(query)
		if err != nil {
			respond.Error(w, err)
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
		if matched == nil || !MembershipAllowsShipperShipmentRead(matched.CompanyType, matched.RoleCodes) {
			respond.Error(w, apperrors.Forbidden("company is not authorized for shipper shipment access"))
			return
		}

		canonical := requestedCompanyID.String()
		query.Set(shipperCompanyQueryKey, canonical)
		r.URL.RawQuery = query.Encode()
		r.Header.Set(HeaderCompanyID, canonical)
		r.Header.Set(HeaderActorKind, ActorBuyer)
		next.ServeHTTP(w, r)
	}
}

// RequireOperatorShipmentRead keeps the legacy tenant-wide shipment GET on
// the existing operator surface. Only a tenant-global PLATFORM_ADMIN role
// may pass. Customer shipper memberships are not an operator context.
func (e *Enforcer) RequireOperatorShipmentRead(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		tenantRoles, err := e.client.ListUserTenantRoles(r.Context(), reqCtx, reqCtx.UserID)
		if err != nil {
			respond.Error(w, membershipLookupError(err))
			return
		}
		if !hasTenantPlatformAdmin(tenantRoles) {
			respond.Error(w, apperrors.Forbidden("insufficient permission"))
			return
		}
		next.ServeHTTP(w, r)
	}
}

func parseShipperCompanyQuery(query url.Values) (uuid.UUID, error) {
	values := query[shipperCompanyQueryKey]
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return uuid.Nil, apperrors.Validation(shipperCompanyQueryKey+" is required", map[string]any{"field": shipperCompanyQueryKey})
	}
	var canonical uuid.UUID
	for i, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return uuid.Nil, apperrors.Validation(shipperCompanyQueryKey+" is required", map[string]any{"field": shipperCompanyQueryKey})
		}
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return uuid.Nil, apperrors.Validation("invalid "+shipperCompanyQueryKey, map[string]any{"field": shipperCompanyQueryKey})
		}
		if i == 0 {
			canonical = parsed
			continue
		}
		if parsed != canonical {
			return uuid.Nil, apperrors.Forbidden("company does not match authenticated membership")
		}
	}
	return canonical, nil
}
