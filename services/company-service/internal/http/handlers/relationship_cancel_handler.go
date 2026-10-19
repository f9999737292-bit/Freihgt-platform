package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/respond"
	"github.com/freight-platform/company-service/internal/service"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type relationshipStatusBody struct {
	SourceType          string `json:"source_type"`
	SourceID            string `json:"source_id"`
	SourceTenantID      string `json:"source_tenant_id"`
	RequestingUserID    string `json:"requesting_user_id"`
	RequestingCompanyID string `json:"requesting_company_id"`
}

type relationshipCancelBody struct {
	SourceType          string  `json:"source_type"`
	SourceID            string  `json:"source_id"`
	SourceTenantID      string  `json:"source_tenant_id"`
	RequestingUserID    string  `json:"requesting_user_id"`
	IdempotencyKey      string  `json:"idempotency_key"`
	CancelReason        *string `json:"cancel_reason"`
	TenantID            *string `json:"tenant_id"`
	CompanyID           *string `json:"company_id"`
	RequestingCompanyID *string `json:"requesting_company_id"`
}

func (h *RelationshipHandler) Status(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var body relationshipStatusBody
	if !decodeStrict(w, r, &body) {
		return
	}
	sourceID, err := uuid.Parse(strings.TrimSpace(body.SourceID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	sourceTenant, err := uuid.Parse(strings.TrimSpace(body.SourceTenantID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	userID, err := uuid.Parse(strings.TrimSpace(body.RequestingUserID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	companyID, err := uuid.Parse(strings.TrimSpace(body.RequestingCompanyID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	rows, err := h.service.ListSourceRequests(r.Context(), service.ListSourceRequestsInput{
		ServiceToken: r.Header.Get(service.RelationshipServiceTokenHeader), SourceType: body.SourceType,
		SourceID: sourceID, SourceTenantID: sourceTenant, RequestingUserID: userID, RequestingCompanyID: companyID,
	})
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

func (h *RelationshipHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var body relationshipCancelBody
	if !decodeStrict(w, r, &body) {
		return
	}
	requestID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "relationship_request_id")))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	sourceID, err := uuid.Parse(strings.TrimSpace(body.SourceID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	sourceTenant, err := uuid.Parse(strings.TrimSpace(body.SourceTenantID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	userID, err := uuid.Parse(strings.TrimSpace(body.RequestingUserID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	tenantID, tenantOK := optionalUUID(body.TenantID)
	if !tenantOK {
		respond.Error(w, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict"))
		return
	}
	bodyCompany, companyOK := optionalCompany(body.CompanyID, body.RequestingCompanyID)
	if !companyOK {
		respond.Error(w, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused"))
		return
	}
	if headerCompany := strings.TrimSpace(r.Header.Get("X-Company-ID")); headerCompany != "" && bodyCompany != nil && headerCompany != bodyCompany.String() {
		respond.Error(w, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused"))
		return
	}
	reason := ""
	if body.CancelReason != nil {
		reason = *body.CancelReason
	}
	out, err := h.service.Cancel(r.Context(), service.CancelRelationshipInput{
		ServiceToken: r.Header.Get(service.RelationshipServiceTokenHeader), SourceType: body.SourceType,
		SourceID: sourceID, SourceTenantID: sourceTenant, RequestingUserID: userID, RequestID: requestID,
		IdempotencyKey: body.IdempotencyKey, Reason: reason, BodyTenantID: tenantID, BodyCompanyID: bodyCompany,
		CorrelationID: sharedmiddleware.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{
		"relationship_request_id": out.RequestID.String(),
		"request_status":          out.Status,
	})
}

func decodeStrict(w http.ResponseWriter, r *http.Request, dest any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil && err != io.EOF {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return false
	}
	return true
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
