package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

func resolveVerifiedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("tenant context is required")
	}
	return domain.ParseUUID(raw, "tenant_id")
}

func resolveVerifiedUser(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("user context is required")
	}
	return domain.ParseUUID(raw, "user_id")
}

func parseSettlementActor(r *http.Request) (companyID uuid.UUID, actorKind string, err error) {
	actorKind = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("actor")))
	companyRaw := strings.TrimSpace(r.URL.Query().Get("company_id"))
	if companyRaw == "" {
		return uuid.Nil, "", apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	companyID, err = domain.ParseUUID(companyRaw, "company_id")
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

func settlementActorInput(r *http.Request) (domain.SettlementActorInput, error) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	companyID, actorKind, err := parseSettlementActor(r)
	if err != nil {
		return domain.SettlementActorInput{}, err
	}
	return domain.SettlementActorInput{
		TenantID: tenantID, ActorCompanyID: companyID, ActorKind: actorKind, ActorUserID: userID,
	}, nil
}
