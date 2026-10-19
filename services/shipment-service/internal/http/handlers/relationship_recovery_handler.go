package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/http/userauth"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

const relationshipCancelSuffix = "/cancel"

func (h *RelationshipCreateHandler) Status(w http.ResponseWriter, r *http.Request) {
	noStore(w)
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
	if !ok || h == nil || h.service == nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	rows, err := h.service.Status(r.Context(), service.RelationshipActor{UserID: principal.UserID, TenantID: principal.TenantID}, shipmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]string{
			"relationship_request_id":    row.RequestID.String(),
			"request_status":             row.Status,
			"shipper_carrier_company_id": row.CarrierCompanyID.String(),
			"token_expires_at":           row.TokenExpiresAt.UTC().Format(time.RFC3339Nano),
		})
	}
	respond.JSON(w, http.StatusOK, map[string]any{"requests": items})
}

func (h *RelationshipCreateHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	principal, ok := userauth.PrincipalFromContext(r.Context())
	if !ok {
		respond.Error(w, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required"))
		return
	}
	shipmentID, requestID, ok := ShipmentRelationshipCancelTarget(r.URL.Path)
	if !ok || h == nil || h.service == nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	reason, valid := cancelReason(r)
	if !valid {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	out, err := h.service.Cancel(r.Context(), service.RelationshipActor{UserID: principal.UserID, TenantID: principal.TenantID}, shipmentID, requestID, key, reason)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{
		"relationship_request_id": out.RequestID.String(),
		"request_status":          out.Status,
	})
}

// ShipmentRelationshipCancelTarget reads the shipment and request selectors from the cancel path.
func ShipmentRelationshipCancelTarget(path string) (uuid.UUID, uuid.UUID, bool) {
	const marker = "/carrier-relationship-requests/"
	if !strings.HasPrefix(path, relationshipCreatePrefix) || !strings.HasSuffix(path, relationshipCancelSuffix) {
		return uuid.Nil, uuid.Nil, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(path, relationshipCreatePrefix), relationshipCancelSuffix)
	shipmentRaw, requestRaw, ok := strings.Cut(body, marker)
	if !ok || strings.Contains(shipmentRaw, "/") || strings.Contains(requestRaw, "/") || shipmentRaw == "" || requestRaw == "" {
		return uuid.Nil, uuid.Nil, false
	}
	shipmentID, err := uuid.Parse(shipmentRaw)
	if err != nil || shipmentID == uuid.Nil {
		return uuid.Nil, uuid.Nil, false
	}
	requestID, err := uuid.Parse(requestRaw)
	if err != nil || requestID == uuid.Nil {
		return uuid.Nil, uuid.Nil, false
	}
	return shipmentID, requestID, true
}

func cancelReason(r *http.Request) (string, bool) {
	for key := range r.URL.Query() {
		if relationshipTrustField(key) || strings.EqualFold(key, "cancel_reason") || strings.EqualFold(key, "idempotency_key") {
			return "", false
		}
	}
	if r.Body == nil || r.ContentLength == 0 {
		return "", true
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return "", false
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "", true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", false
	}
	if len(fields) == 0 {
		return "", true
	}
	if len(fields) != 1 {
		return "", false
	}
	rawReason, ok := fields["cancel_reason"]
	if !ok {
		return "", false
	}
	var reason string
	if err := json.Unmarshal(rawReason, &reason); err != nil {
		return "", false
	}
	return reason, true
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
