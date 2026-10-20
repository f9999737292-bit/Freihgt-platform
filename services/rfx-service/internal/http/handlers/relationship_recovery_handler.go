package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/http/userauth"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

func (h *RfxRelationshipCreateHandler) Status(w http.ResponseWriter, r *http.Request) {
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
	sourceType, sourceID, ok := RfxRelationshipCreateTarget(r.URL.Path)
	if !ok || h == nil || h.service == nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	rows, err := h.service.Status(r.Context(), service.RfxRelationshipActor{UserID: principal.UserID, TenantID: principal.TenantID}, sourceType, sourceID)
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

func (h *RfxRelationshipCreateHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	principal, ok := userauth.PrincipalFromContext(r.Context())
	if !ok {
		respond.Error(w, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required"))
		return
	}
	sourceType, sourceID, requestID, ok := RfxRelationshipCancelTarget(r.URL.Path)
	if !ok || h == nil || h.service == nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	reason, valid := cancelReason(r)
	if !valid {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	out, err := h.service.Cancel(r.Context(), service.RfxRelationshipActor{UserID: principal.UserID, TenantID: principal.TenantID}, sourceType, sourceID, requestID, strings.TrimSpace(r.Header.Get("Idempotency-Key")), reason)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{
		"relationship_request_id": out.RequestID.String(),
		"request_status":          out.Status,
	})
}

// RfxRelationshipCancelTarget reads the source and request selectors from a cancel path.
func RfxRelationshipCancelTarget(path string) (string, uuid.UUID, uuid.UUID, bool) {
	const marker = "/carrier-relationship-requests/"
	prefix := ""
	sourceType := ""
	switch {
	case strings.HasPrefix(path, "/v1/rfx-awards/"):
		prefix = "/v1/rfx-awards/"
		sourceType = domain.RfxSourceTypeAward
	case strings.HasPrefix(path, "/v1/rfx-award-transport-orders/"):
		prefix = "/v1/rfx-award-transport-orders/"
		sourceType = domain.RfxSourceTypeAwardLink
	default:
		return "", uuid.Nil, uuid.Nil, false
	}
	if !strings.HasSuffix(path, "/cancel") {
		return "", uuid.Nil, uuid.Nil, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/cancel")
	sourceRaw, requestRaw, found := strings.Cut(body, marker)
	if !found || sourceRaw == "" || requestRaw == "" || strings.Contains(sourceRaw, "/") || strings.Contains(requestRaw, "/") {
		return "", uuid.Nil, uuid.Nil, false
	}
	sourceID, err := uuid.Parse(sourceRaw)
	if err != nil || sourceID == uuid.Nil {
		return "", uuid.Nil, uuid.Nil, false
	}
	requestID, err := uuid.Parse(requestRaw)
	if err != nil || requestID == uuid.Nil {
		return "", uuid.Nil, uuid.Nil, false
	}
	return sourceType, sourceID, requestID, true
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
	if err := json.Unmarshal(raw, &fields); err != nil || (len(fields) != 0 && (len(fields) != 1 || fields["cancel_reason"] == nil)) {
		return "", false
	}
	if len(fields) == 0 {
		return "", true
	}
	var reason string
	if err := json.Unmarshal(fields["cancel_reason"], &reason); err != nil {
		return "", false
	}
	return reason, true
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
