package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/service"
)

// ShipmentSourceAuthorityHandler is the internal owner callback.
// The response body is only the decision. It does not echo shipment fields.
type ShipmentSourceAuthorityHandler struct {
	authority *service.ShipmentSourceAuthority
}

func NewShipmentSourceAuthorityHandler(authority *service.ShipmentSourceAuthority) *ShipmentSourceAuthorityHandler {
	return &ShipmentSourceAuthorityHandler{authority: authority}
}

type shipmentSourceBody struct {
	SourceType              string `json:"source_type"`
	SourceID                string `json:"source_id"`
	SourceTenantID          string `json:"source_tenant_id"`
	Fingerprint             string `json:"source_version_or_fingerprint"`
	ShipperCarrierCompanyID string `json:"shipper_carrier_company_id"`
	RequestingCompanyID     string `json:"requesting_company_id"`
	RequestingUserID        string `json:"requesting_user_id"`
}

func (h *ShipmentSourceAuthorityHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeShipmentSourceBody(r)
	if !ok {
		writeShipmentSourceDecision(w, http.StatusServiceUnavailable, domain.ShipmentSourceCreateUnavailable)
		return
	}
	subject, ok := confirmSubject(body)
	if !ok {
		writeShipmentSourceDecision(w, http.StatusServiceUnavailable, domain.ShipmentSourceCreateUnavailable)
		return
	}
	decision := domain.ShipmentSourceCreateUnavailable
	if h != nil && h.authority != nil {
		decision = h.authority.Confirm(r.Context(), subject)
	}
	status := http.StatusOK
	if decision == domain.ShipmentSourceCreateUnavailable {
		status = http.StatusServiceUnavailable
	}
	writeShipmentSourceDecision(w, status, decision)
}

func (h *ShipmentSourceAuthorityHandler) Revalidate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeShipmentSourceBody(r)
	if !ok {
		writeShipmentSourceDecision(w, http.StatusServiceUnavailable, domain.ShipmentSourceUnavailable)
		return
	}
	snap, ok := revalidateSnapshot(body)
	if !ok {
		writeShipmentSourceDecision(w, http.StatusServiceUnavailable, domain.ShipmentSourceUnavailable)
		return
	}
	decision := domain.ShipmentSourceUnavailable
	if h != nil && h.authority != nil {
		decision = h.authority.Revalidate(r.Context(), snap)
	}
	status := http.StatusOK
	if decision == domain.ShipmentSourceUnavailable {
		status = http.StatusServiceUnavailable
	}
	writeShipmentSourceDecision(w, status, decision)
}

func decodeShipmentSourceBody(r *http.Request) (shipmentSourceBody, bool) {
	var body shipmentSourceBody
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return shipmentSourceBody{}, false
	}
	return body, true
}

func confirmSubject(body shipmentSourceBody) (domain.ShipmentSourceSubject, bool) {
	sourceID, ok := parseSourceUUID(body.SourceID)
	tenantID, tenantOK := parseSourceUUID(body.SourceTenantID)
	carrierID, carrierOK := parseSourceUUID(body.ShipperCarrierCompanyID)
	companyID, companyOK := parseSourceUUID(body.RequestingCompanyID)
	userID, userOK := parseSourceUUID(body.RequestingUserID)
	if !ok || !tenantOK || !carrierOK || !companyOK || !userOK {
		return domain.ShipmentSourceSubject{}, false
	}
	return domain.ShipmentSourceSubject{
		SourceType: strings.TrimSpace(body.SourceType), SourceID: sourceID, SourceTenantID: tenantID,
		Fingerprint: strings.TrimSpace(body.Fingerprint), ShipperCarrierCompanyID: carrierID,
		RequestingCompanyID: companyID, RequestingUserID: userID,
	}, true
}

func revalidateSnapshot(body shipmentSourceBody) (domain.ShipmentSourceSnapshot, bool) {
	sourceID, ok := parseSourceUUID(body.SourceID)
	tenantID, tenantOK := parseSourceUUID(body.SourceTenantID)
	carrierID, carrierOK := parseSourceUUID(body.ShipperCarrierCompanyID)
	if !ok || !tenantOK || !carrierOK {
		return domain.ShipmentSourceSnapshot{}, false
	}
	return domain.ShipmentSourceSnapshot{
		SourceType: strings.TrimSpace(body.SourceType), SourceID: sourceID, SourceTenantID: tenantID,
		Fingerprint: strings.TrimSpace(body.Fingerprint), ShipperCarrierCompanyID: carrierID,
	}, true
}

func parseSourceUUID(value string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, false
	}
	return parsed, true
}

func writeShipmentSourceDecision(w http.ResponseWriter, status int, decision string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"decision": decision})
}
