package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
)

func resolveRequestActor(r *http.Request) (domain.ActorContext, error) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		return domain.ActorContext{}, err
	}
	var userID uuid.UUID
	if raw := strings.TrimSpace(r.Header.Get("X-User-ID")); raw != "" {
		userID, err = domain.ParseUUID(raw, "user_id")
		if err != nil {
			return domain.ActorContext{}, err
		}
	}
	return domain.ActorContext{TenantID: tenantID, UserID: userID}, nil
}

func rejectClientTenantQuery(r *http.Request) error {
	if strings.TrimSpace(r.URL.Query().Get("tenant_id")) != "" {
		return apperrors.Forbidden("tenant_id query parameter is not accepted")
	}
	return nil
}

func rejectBodyTenantMismatch(bodyTenant string, trusted uuid.UUID) error {
	bodyTenant = strings.TrimSpace(bodyTenant)
	if bodyTenant == "" {
		return nil
	}
	parsed, err := domain.ParseUUID(bodyTenant, "tenant_id")
	if err != nil {
		return err
	}
	if parsed != trusted {
		return apperrors.Forbidden("tenant_id in request body does not match authenticated tenant")
	}
	return nil
}

func requireActor(w http.ResponseWriter, r *http.Request) (domain.ActorContext, bool) {
	if err := rejectClientTenantQuery(r); err != nil {
		respond.Error(w, err)
		return domain.ActorContext{}, false
	}
	actor, err := resolveRequestActor(r)
	if err != nil {
		respond.Error(w, err)
		return domain.ActorContext{}, false
	}
	return actor, true
}
