package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/http/userauth"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

const (
	relationshipCreatePrefix = "/v1/shipments/"
	relationshipCreateSuffix = "/carrier-relationship-requests"
)

// RelationshipCreator is the shipment relationship create command.
type RelationshipCreator interface {
	Create(ctx context.Context, actor service.RelationshipActor, shipmentID uuid.UUID, idempotencyKey string) (service.RelationshipCreateResult, error)
}

// RelationshipCreateHandler is the authenticated shipper command.
// The shipment id is a selector. Tenant, fingerprint, and service identity are not accepted from the client.
// The acceptance token is copied into the JSON body only when this call created the request.
type RelationshipCreateHandler struct {
	service RelationshipCreator
}

func NewRelationshipCreateHandler(svc RelationshipCreator) *RelationshipCreateHandler {
	return &RelationshipCreateHandler{service: svc}
}

func (h *RelationshipCreateHandler) Create(w http.ResponseWriter, r *http.Request) {
	principal, ok := userauth.PrincipalFromContext(r.Context())
	if !ok {
		respond.Error(w, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required"))
		return
	}
	if clientSuppliedRelationshipTrust(r) {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	shipmentID, ok := ShipmentRelationshipCreateID(r.URL.Path)
	if !ok {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	if h == nil || h.service == nil {
		respond.Error(w, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable"))
		return
	}
	out, err := h.service.Create(r.Context(), service.RelationshipActor{
		UserID: principal.UserID, TenantID: principal.TenantID,
	}, shipmentID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusOK
	payload := map[string]any{
		"relationship_request_id": out.RequestID.String(),
		"request_status":          out.Status,
	}
	if out.Created {
		status = http.StatusCreated
		payload["acceptance_token"] = out.AcceptanceToken
	}
	respond.JSON(w, status, payload)
}

// ShipmentRelationshipCreateID reads the shipment selector from the create path.
func ShipmentRelationshipCreateID(path string) (uuid.UUID, bool) {
	if !strings.HasPrefix(path, relationshipCreatePrefix) || !strings.HasSuffix(path, relationshipCreateSuffix) {
		return uuid.Nil, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, relationshipCreatePrefix), relationshipCreateSuffix)
	if raw == "" || strings.Contains(raw, "/") {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

func clientSuppliedRelationshipTrust(r *http.Request) bool {
	for key := range r.URL.Query() {
		if relationshipTrustField(key) {
			return true
		}
	}
	if r.Body == nil || r.ContentLength == 0 {
		return false
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return true
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) > 0 {
		return true
	}
	return false
}

func relationshipTrustField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "tenant_id", "source_tenant_id", "source_id", "source_type", "source_version_or_fingerprint",
		"fingerprint", "shipper_carrier_company_id", "requesting_user_id", "requesting_company_id",
		"company_id", "acceptance_token", "service_name", "x-internal-service-name":
		return true
	default:
		return false
	}
}
