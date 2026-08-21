package security

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

const (
	ActorKindBuyer  = "BUYER"
	ActorKindCarrier = "CARRIER"
)

type TrustedActor struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	CompanyID uuid.UUID
	ActorKind string
}

func ParseTrustedActor(r *http.Request) (TrustedActor, error) {
	tenantRaw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	userRaw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	companyRaw := strings.TrimSpace(r.Header.Get("X-Company-ID"))
	kindRaw := strings.TrimSpace(r.Header.Get("X-Actor-Kind"))

	if tenantRaw == "" {
		return TrustedActor{}, apperrors.Validation("tenant context is required", map[string]any{"field": "tenant_id"})
	}
	if userRaw == "" {
		return TrustedActor{}, apperrors.Validation("user context is required", map[string]any{"field": "user_id"})
	}
	if companyRaw == "" {
		return TrustedActor{}, apperrors.Validation("company context is required", map[string]any{"field": "company_id"})
	}
	if kindRaw == "" {
		return TrustedActor{}, apperrors.Validation("actor kind is required", map[string]any{"field": "actor_kind"})
	}

	tenantID, err := uuid.Parse(tenantRaw)
	if err != nil || tenantID == uuid.Nil {
		return TrustedActor{}, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenant_id"})
	}
	userID, err := uuid.Parse(userRaw)
	if err != nil || userID == uuid.Nil {
		return TrustedActor{}, apperrors.Validation("invalid user id", map[string]any{"field": "user_id"})
	}
	companyID, err := uuid.Parse(companyRaw)
	if err != nil || companyID == uuid.Nil {
		return TrustedActor{}, apperrors.Validation("invalid company id", map[string]any{"field": "company_id"})
	}

	kind := strings.ToUpper(kindRaw)
	switch kind {
	case ActorKindBuyer, ActorKindCarrier:
	default:
		return TrustedActor{}, apperrors.Validation("actor kind must be BUYER or CARRIER", map[string]any{"field": "actor_kind"})
	}

	return TrustedActor{
		TenantID:  tenantID,
		UserID:    userID,
		CompanyID: companyID,
		ActorKind: kind,
	}, nil
}
