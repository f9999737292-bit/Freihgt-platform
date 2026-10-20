package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	"github.com/freight-platform/company-service/internal/http/relationshipauth"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/respond"
	"github.com/freight-platform/company-service/internal/service"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type RelationshipHandler struct {
	service *service.RelationshipService
}

func NewRelationshipHandler(svc *service.RelationshipService) *RelationshipHandler {
	return &RelationshipHandler{service: svc}
}

type relationshipWriteBody struct {
	SourceType                  string  `json:"source_type"`
	SourceID                    string  `json:"source_id"`
	SourceTenantID              *string `json:"source_tenant_id"`
	Fingerprint                 string  `json:"source_version_or_fingerprint"`
	ShipperCarrierCompanyID     string  `json:"shipper_carrier_company_id"`
	RequestingUserID            *string `json:"requesting_user_id"`
	IdempotencyKey              string  `json:"idempotency_key"`
	AcceptanceToken             string  `json:"acceptance_token"`
	RelationshipRequestID       string  `json:"relationship_request_id"`
	TenantID                    *string `json:"tenant_id"`
	CompanyID                   *string `json:"company_id"`
	RequestingCompanyID         *string `json:"requesting_company_id"`
	Reason                      string  `json:"reason"`
	ExpectedRelationshipVersion *int    `json:"expected_relationship_version"`
}

func (h *RelationshipHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body relationshipWriteBody
	if !decodeRelationshipBody(w, r, &body) {
		return
	}
	sourceID, err := uuid.Parse(strings.TrimSpace(body.SourceID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	carrierID, err := uuid.Parse(strings.TrimSpace(body.ShipperCarrierCompanyID))
	if err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	requestingUser, userOK := optionalUUID(body.RequestingUserID)
	sourceTenant, sourceTenantOK := optionalUUID(body.SourceTenantID)
	if !userOK || requestingUser == nil || !sourceTenantOK || sourceTenant == nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return
	}
	tenantID, tenantOK := optionalUUID(body.TenantID)
	if !tenantOK {
		respond.Error(w, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict"))
		return
	}
	bodyCompany, companyOK := optionalCompany(body.CompanyID, body.RequestingCompanyID)
	if !companyOK || bodyCompany == nil {
		respond.Error(w, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused"))
		return
	}
	if headerCompany := strings.TrimSpace(r.Header.Get("X-Company-ID")); headerCompany != "" && headerCompany != bodyCompany.String() {
		respond.Error(w, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused"))
		return
	}
	out, err := h.service.Create(r.Context(), service.CreateRelationshipInput{
		ServiceToken:      r.Header.Get(service.RelationshipServiceTokenHeader),
		Principal:         domain.Principal{UserID: *requestingUser, TenantID: *sourceTenant},
		SelectedCompanyID: *bodyCompany, SourceType: strings.TrimSpace(body.SourceType),
		SourceID: sourceID, Fingerprint: body.Fingerprint, ShipperCarrierCompanyID: carrierID,
		IdempotencyKey: body.IdempotencyKey, BodyTenantID: tenantID, BodyCompanyID: bodyCompany,
		CorrelationID: sharedmiddleware.RequestIDFromContext(r.Context()),
	})
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

func (h *RelationshipHandler) Accept(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	principal, companyID, ok := relationshipActor(w, r)
	if !ok {
		return
	}
	var body relationshipWriteBody
	if !decodeRelationshipBody(w, r, &body) {
		return
	}
	var requestID *uuid.UUID
	locatorInvalid := false
	if strings.TrimSpace(body.RelationshipRequestID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(body.RelationshipRequestID))
		if err != nil {
			locatorInvalid = true
		} else {
			requestID = &parsed
		}
	}
	tenantID, tenantOK := optionalUUID(body.TenantID)
	if !tenantOK {
		respond.Error(w, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict"))
		return
	}
	bodyCompany, companyOK := optionalCompany(body.CompanyID, nil)
	if !companyOK {
		respond.Error(w, apperrors.New(apperrors.CodeRelationshipAcceptanceForbidden, "relationship command refused"))
		return
	}
	out, err := h.service.Accept(r.Context(), service.AcceptRelationshipInput{
		Principal: principal, SelectedCompanyID: companyID, AcceptanceToken: body.AcceptanceToken,
		IdempotencyKey: body.IdempotencyKey, RequestID: requestID, LocatorInvalid: locatorInvalid, BodyTenantID: tenantID, BodyCompanyID: bodyCompany,
		CorrelationID: sharedmiddleware.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusOK
	if out.Created {
		status = http.StatusCreated
	}
	respond.JSON(w, status, map[string]any{
		"relationship_id":      out.RelationshipID.String(),
		"relationship_version": out.RelationshipVersion,
		"request_status":       out.RequestStatus,
	})
}

func (h *RelationshipHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	principal, companyID, ok := relationshipActor(w, r)
	if !ok {
		return
	}
	relationshipID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "relationship_id")))
	if err != nil {
		relationshipID = uuid.Nil
	}
	var body relationshipWriteBody
	if !decodeRelationshipBody(w, r, &body) {
		return
	}
	tenantID, tenantOK := optionalUUID(body.TenantID)
	if !tenantOK {
		respond.Error(w, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict"))
		return
	}
	bodyCompany, companyOK := optionalCompany(body.CompanyID, nil)
	if !companyOK {
		respond.Error(w, apperrors.New(apperrors.CodeRelationshipRevokeForbidden, "relationship command refused"))
		return
	}
	out, err := h.service.Revoke(r.Context(), service.RevokeRelationshipInput{
		Principal: principal, SelectedCompanyID: companyID, RelationshipID: relationshipID,
		IdempotencyKey: body.IdempotencyKey, Reason: body.Reason, AcceptanceToken: body.AcceptanceToken,
		ExpectedRelationshipVersion: body.ExpectedRelationshipVersion, BodyTenantID: tenantID, BodyCompanyID: bodyCompany,
		CorrelationID: sharedmiddleware.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"relationship_id":      out.RelationshipID.String(),
		"relationship_version": out.RelationshipVersion,
		"status":               out.Status,
	})
}

func (h *RelationshipHandler) Get(w http.ResponseWriter, r *http.Request) {
	principal, companyID, ok := relationshipActor(w, r)
	if !ok {
		return
	}
	requestID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "relationship_request_id")))
	if err != nil {
		requestID = uuid.Nil
	}
	view, err := h.service.Get(r.Context(), principal, companyID, requestID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"relationship_request_id": view.RequestID.String(),
		"request_status":          view.Status,
	})
}

func relationshipActor(w http.ResponseWriter, r *http.Request) (domain.Principal, uuid.UUID, bool) {
	principal, ok := relationshipauth.PrincipalFromContext(r.Context())
	if !ok {
		respond.Error(w, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required"))
		return domain.Principal{}, uuid.Nil, false
	}
	companyID, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Company-ID")))
	if err != nil {
		code := apperrors.CodeRelationshipRequestForbidden
		switch {
		case strings.Contains(r.URL.Path, "acceptances"):
			code = apperrors.CodeRelationshipAcceptanceForbidden
		case strings.Contains(r.URL.Path, "revocations"):
			code = apperrors.CodeRelationshipRevokeForbidden
		}
		respond.Error(w, apperrors.New(code, "relationship command refused"))
		return domain.Principal{}, uuid.Nil, false
	}
	return principal, companyID, true
}

func decodeRelationshipBody(w http.ResponseWriter, r *http.Request, body *relationshipWriteBody) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(body); err != nil {
		respond.Error(w, apperrors.Validation("relationship request is invalid", nil))
		return false
	}
	return true
}

func optionalUUID(value *string) (*uuid.UUID, bool) {
	if value == nil {
		return nil, true
	}
	parsed, err := uuid.Parse(strings.TrimSpace(*value))
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func optionalCompany(values ...*string) (*uuid.UUID, bool) {
	var chosen *uuid.UUID
	for _, value := range values {
		parsed, ok := optionalUUID(value)
		if !ok {
			return nil, false
		}
		if parsed == nil {
			continue
		}
		if chosen != nil && *chosen != *parsed {
			return nil, false
		}
		chosen = parsed
	}
	return chosen, true
}

func relationshipNotFound() error {
	return apperrors.New(apperrors.CodeRelationshipRequestNotFound, "relationship request not found")
}
