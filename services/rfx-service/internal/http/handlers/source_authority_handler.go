package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/service"
)

// RfxSourceAuthorityHandler is the internal owner callback.
// The response body is only the decision. It does not echo award or link fields.
type RfxSourceAuthorityHandler struct {
	authority *service.RfxSourceAuthority
}

func NewRfxSourceAuthorityHandler(authority *service.RfxSourceAuthority) *RfxSourceAuthorityHandler {
	return &RfxSourceAuthorityHandler{authority: authority}
}

type rfxSourceBody struct {
	SourceType              string `json:"source_type"`
	SourceID                string `json:"source_id"`
	SourceTenantID          string `json:"source_tenant_id"`
	Fingerprint             string `json:"source_version_or_fingerprint"`
	ShipperCarrierCompanyID string `json:"shipper_carrier_company_id"`
	RequestingCompanyID     string `json:"requesting_company_id"`
	RequestingUserID        string `json:"requesting_user_id"`
}

func (h *RfxSourceAuthorityHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeRfxSourceBody(r)
	if !ok {
		writeRfxSourceDecision(w, http.StatusServiceUnavailable, domain.RfxSourceCreateUnavailable)
		return
	}
	subject, ok := confirmRfxSubject(body)
	if !ok {
		writeRfxSourceDecision(w, http.StatusServiceUnavailable, domain.RfxSourceCreateUnavailable)
		return
	}
	decision := domain.RfxSourceCreateUnavailable
	if h != nil && h.authority != nil {
		decision = h.authority.Confirm(r.Context(), subject)
	}
	status := http.StatusOK
	if decision == domain.RfxSourceCreateUnavailable {
		status = http.StatusServiceUnavailable
	}
	writeRfxSourceDecision(w, status, decision)
}

func (h *RfxSourceAuthorityHandler) Revalidate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeRfxSourceBody(r)
	if !ok {
		writeRfxSourceDecision(w, http.StatusServiceUnavailable, domain.RfxSourceUnavailable)
		return
	}
	snap, ok := revalidateRfxSnapshot(body)
	if !ok {
		writeRfxSourceDecision(w, http.StatusServiceUnavailable, domain.RfxSourceUnavailable)
		return
	}
	decision := domain.RfxSourceUnavailable
	if h != nil && h.authority != nil {
		decision = h.authority.Revalidate(r.Context(), snap)
	}
	status := http.StatusOK
	if decision == domain.RfxSourceUnavailable {
		status = http.StatusServiceUnavailable
	}
	writeRfxSourceDecision(w, status, decision)
}

func decodeRfxSourceBody(r *http.Request) (rfxSourceBody, bool) {
	var body rfxSourceBody
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return rfxSourceBody{}, false
	}
	return body, true
}

func confirmRfxSubject(body rfxSourceBody) (domain.RfxSourceSubject, bool) {
	sourceID, ok := parseRfxSourceUUID(body.SourceID)
	tenantID, tenantOK := parseRfxSourceUUID(body.SourceTenantID)
	carrierID, carrierOK := parseRfxSourceUUID(body.ShipperCarrierCompanyID)
	companyID, companyOK := parseRfxSourceUUID(body.RequestingCompanyID)
	userID, userOK := parseRfxSourceUUID(body.RequestingUserID)
	if !ok || !tenantOK || !carrierOK || !companyOK || !userOK {
		return domain.RfxSourceSubject{}, false
	}
	return domain.RfxSourceSubject{
		SourceType: strings.TrimSpace(body.SourceType), SourceID: sourceID, SourceTenantID: tenantID,
		Fingerprint: strings.TrimSpace(body.Fingerprint), ShipperCarrierCompanyID: carrierID,
		RequestingCompanyID: companyID, RequestingUserID: userID,
	}, true
}

func revalidateRfxSnapshot(body rfxSourceBody) (domain.RfxSourceSnapshot, bool) {
	sourceID, ok := parseRfxSourceUUID(body.SourceID)
	tenantID, tenantOK := parseRfxSourceUUID(body.SourceTenantID)
	carrierID, carrierOK := parseRfxSourceUUID(body.ShipperCarrierCompanyID)
	if !ok || !tenantOK || !carrierOK {
		return domain.RfxSourceSnapshot{}, false
	}
	return domain.RfxSourceSnapshot{
		SourceType: strings.TrimSpace(body.SourceType), SourceID: sourceID, SourceTenantID: tenantID,
		Fingerprint: strings.TrimSpace(body.Fingerprint), ShipperCarrierCompanyID: carrierID,
	}, true
}

func parseRfxSourceUUID(value string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, false
	}
	return parsed, true
}

func writeRfxSourceDecision(w http.ResponseWriter, status int, decision string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"decision": decision})
}
