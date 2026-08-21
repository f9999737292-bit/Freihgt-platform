package contractrates

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/ratesrbac"
)

var forbiddenSimulationFields = []string{
	"manual_spot_amount",
	"manual_spot_currency",
	"pricing_source",
	"award_link_id",
	"award_scope_event_id",
	"award_scope_lot_id",
	"bid_id",
}

type publicResolveRequest struct {
	BuyerCompanyID        uuid.UUID `json:"buyer_company_id"`
	CarrierCompanyID      uuid.UUID `json:"carrier_company_id"`
	OriginLocationID      uuid.UUID `json:"origin_location_id"`
	DestinationLocationID uuid.UUID `json:"destination_location_id"`
	EquipmentType         string    `json:"equipment_type"`
	TransportMode         string    `json:"transport_mode"`
	PricingDate           string    `json:"pricing_date,omitempty"`
	CurrencyCode          *string   `json:"currency_code,omitempty"`
}

func validateAndRebuildBody(method, path string, raw []byte, vc ratesrbac.VerifiedContext) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}

	switch {
	case method == "POST" && path == "/api/v1/rates/resolve":
		return validateResolveBody(raw, vc)
	case method == "POST" && path == "/api/v1/transport-contracts":
		return validateCreateContractBody(raw, vc)
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/transport-contracts/"):
		return validatePatchContractBody(raw)
	case method == "POST" && strings.HasSuffix(path, "/terminate"):
		return validateTerminateBody(raw)
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/rate-card-versions/"):
		return validatePatchRateVersionBody(raw)
	default:
		return validateStrictObject(raw)
	}
}

func validateStrictObject(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	if dec.More() {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	return raw, nil
}

func validateResolveBody(raw []byte, vc ratesrbac.VerifiedContext) ([]byte, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	for _, field := range forbiddenSimulationFields {
		if _, ok := probe[field]; ok {
			return nil, apperrors.Validation("field is not allowed on public simulation", map[string]any{"field": field})
		}
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var req publicResolveRequest
	if err := dec.Decode(&req); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}

	selectedCompany, err := uuid.Parse(vc.CompanyID)
	if err != nil {
		return nil, apperrors.Forbidden("verified company context is required")
	}
	if vc.ActorKind == "BUYER" && req.BuyerCompanyID != selectedCompany {
		return nil, apperrors.Forbidden("buyer_company_id does not match selected company")
	}
	if vc.ActorKind == "CARRIER" && req.CarrierCompanyID != selectedCompany {
		return nil, apperrors.Forbidden("carrier_company_id does not match selected company")
	}

	out, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type publicCreateContractRequest struct {
	BuyerCompanyID    uuid.UUID `json:"buyer_company_id"`
	CarrierCompanyID  uuid.UUID `json:"carrier_company_id"`
	ContractNumber    string    `json:"contract_number"`
	ExternalReference *string   `json:"external_reference"`
	Name              string    `json:"name"`
	Description       *string   `json:"description"`
	ValidFrom         string    `json:"valid_from"`
	ValidTo           *string   `json:"valid_to"`
	CurrencyCode      string    `json:"currency_code"`
}

func validateCreateContractBody(raw []byte, vc ratesrbac.VerifiedContext) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var req publicCreateContractRequest
	if err := dec.Decode(&req); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	selectedCompany, err := uuid.Parse(vc.CompanyID)
	if err != nil {
		return nil, apperrors.Forbidden("verified company context is required")
	}
	if req.BuyerCompanyID != selectedCompany {
		return nil, apperrors.Forbidden("buyer_company_id must match selected company")
	}
	out, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type publicPatchContractRequest struct {
	Name              *string         `json:"name"`
	Description       *string         `json:"description"`
	ExternalReference *string         `json:"external_reference"`
	ValidTo           json.RawMessage `json:"valid_to"`
}

func validatePatchContractBody(raw []byte) ([]byte, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	allowed := map[string]struct{}{
		"name": {}, "description": {}, "external_reference": {}, "valid_to": {},
	}
	for key := range probe {
		if _, ok := allowed[key]; !ok {
			return nil, apperrors.Validation("unknown field", map[string]any{"field": key})
		}
	}
	if rawValidTo, ok := probe["valid_to"]; ok {
		if err := validateNullableDateJSON(rawValidTo, "valid_to"); err != nil {
			return nil, err
		}
	}
	return raw, nil
}

type publicPatchRateVersionRequest struct {
	ValidFrom json.RawMessage `json:"valid_from"`
	ValidTo   json.RawMessage `json:"valid_to"`
}

func validatePatchRateVersionBody(raw []byte) ([]byte, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	allowed := map[string]struct{}{"valid_from": {}, "valid_to": {}}
	for key := range probe {
		if _, ok := allowed[key]; !ok {
			return nil, apperrors.Validation("unknown field", map[string]any{"field": key})
		}
	}
	if rawValidFrom, ok := probe["valid_from"]; ok && len(rawValidFrom) > 0 {
		if err := validateDateStringJSON(rawValidFrom, "valid_from"); err != nil {
			return nil, err
		}
	}
	if rawValidTo, ok := probe["valid_to"]; ok {
		if err := validateNullableDateJSON(rawValidTo, "valid_to"); err != nil {
			return nil, err
		}
	}
	return raw, nil
}

type publicTerminateRequest struct {
	TerminationReason *string `json:"termination_reason"`
}

func validateTerminateBody(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var req publicTerminateRequest
	if err := dec.Decode(&req); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	out, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func validateNullableDateJSON(raw json.RawMessage, field string) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	if string(raw) == "null" {
		return nil
	}
	return validateDateStringJSON(raw, field)
}

func validateDateStringJSON(raw json.RawMessage, field string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return apperrors.Validation("invalid date format, expected YYYY-MM-DD or null", map[string]any{"field": field})
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return apperrors.Validation("invalid date format, expected YYYY-MM-DD", map[string]any{"field": field})
	}
	parts := strings.Split(value, "-")
	if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		return apperrors.Validation("invalid date format, expected YYYY-MM-DD", map[string]any{"field": field})
	}
	return nil
}
