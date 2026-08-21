package contractrates

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/ratesrbac"
)

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

type publicCreateContractRequest struct {
	BuyerCompanyID    uuid.UUID `json:"buyer_company_id"`
	CarrierCompanyID  uuid.UUID `json:"carrier_company_id"`
	ContractNumber    string    `json:"contract_number"`
	ExternalReference *string   `json:"external_reference,omitempty"`
	Name              string    `json:"name"`
	Description       *string   `json:"description,omitempty"`
	ValidFrom         string    `json:"valid_from"`
	ValidTo           *string   `json:"valid_to,omitempty"`
	CurrencyCode      string    `json:"currency_code"`
}

type publicPatchContractRequest struct {
	Name              *string         `json:"name,omitempty"`
	Description       *string         `json:"description,omitempty"`
	ExternalReference *string         `json:"external_reference,omitempty"`
	ValidTo           json.RawMessage `json:"valid_to,omitempty"`
}

type publicCreateRateCardRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type publicCreateRateVersionRequest struct {
	ValidFrom string          `json:"valid_from"`
	ValidTo   json.RawMessage `json:"valid_to,omitempty"`
}

type publicPatchRateVersionRequest struct {
	ValidFrom json.RawMessage `json:"valid_from,omitempty"`
	ValidTo   json.RawMessage `json:"valid_to,omitempty"`
}

type publicCreateRateLineRequest struct {
	OriginLocationID      uuid.UUID `json:"origin_location_id"`
	DestinationLocationID uuid.UUID `json:"destination_location_id"`
	EquipmentType         string    `json:"equipment_type"`
	TransportMode         string    `json:"transport_mode"`
}

type publicPatchRateLineRequest struct {
	OriginLocationID      *uuid.UUID `json:"origin_location_id,omitempty"`
	DestinationLocationID *uuid.UUID `json:"destination_location_id,omitempty"`
	EquipmentType         *string    `json:"equipment_type,omitempty"`
	TransportMode         *string    `json:"transport_mode,omitempty"`
}

type publicCreateRateComponentRequest struct {
	ComponentType     string  `json:"component_type"`
	CalculationMethod string  `json:"calculation_method"`
	Amount            *string `json:"amount,omitempty"`
	PercentValue      *string `json:"percent_value,omitempty"`
	UnitCode          *string `json:"unit_code,omitempty"`
}

type publicPatchRateComponentRequest struct {
	Amount       *string `json:"amount,omitempty"`
	PercentValue *string `json:"percent_value,omitempty"`
	UnitCode     *string `json:"unit_code,omitempty"`
}

type publicTerminateRequest struct {
	TerminationReason *string `json:"termination_reason,omitempty"`
}

func validateAndRebuildBody(method, path string, raw []byte, vc ratesrbac.VerifiedContext) ([]byte, error) {
	path = strings.TrimSuffix(path, "/")

	switch {
	case method == "POST" && strings.HasSuffix(path, "/terminate"):
		return validateTerminateBody(raw)
	case isContractLifecycle(method, path) || isRateVersionActivate(method, path):
		return validateLifecycleBody(raw)
	case method == "POST" && path == "/api/v1/rates/resolve":
		return validateResolveBody(raw, vc)
	case method == "POST" && path == "/api/v1/transport-contracts":
		return validateCreateContractBody(raw, vc)
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/transport-contracts/") && pathSegmentCount(path) == 2:
		return validatePatchContractBody(raw)
	case method == "POST" && strings.HasSuffix(path, "/rate-cards") && strings.Contains(path, "/transport-contracts/"):
		return validateCreateRateCardBody(raw)
	case method == "POST" && strings.HasSuffix(path, "/versions") && strings.HasPrefix(path, "/api/v1/rate-cards/"):
		return validateCreateRateVersionBody(raw)
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/rate-card-versions/") && pathSegmentCount(path) == 2:
		return validatePatchRateVersionBody(raw)
	case method == "POST" && strings.HasSuffix(path, "/rate-lines") && strings.HasPrefix(path, "/api/v1/rate-card-versions/"):
		return validateCreateRateLineBody(raw)
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/rate-lines/") && pathSegmentCount(path) == 2:
		return validatePatchRateLineBody(raw)
	case method == "POST" && strings.HasSuffix(path, "/components") && strings.HasPrefix(path, "/api/v1/rate-lines/"):
		return validateCreateRateComponentBody(raw)
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/rate-components/") && pathSegmentCount(path) == 2:
		return validatePatchRateComponentBody(raw)
	default:
		if len(bytes.TrimSpace(raw)) > 0 {
			return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
		}
		return raw, nil
	}
}

func pathSegmentCount(path string) int {
	rest := strings.TrimPrefix(path, "/api/v1/")
	return len(strings.Split(strings.Trim(rest, "/"), "/"))
}

func isContractLifecycle(method, path string) bool {
	if method != "POST" || !strings.HasPrefix(path, "/api/v1/transport-contracts/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/transport-contracts/"), "/")
	return len(parts) == 2 && isLifecycleAction(parts[1])
}

func isRateVersionActivate(method, path string) bool {
	if method != "POST" || !strings.HasPrefix(path, "/api/v1/rate-card-versions/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/rate-card-versions/"), "/")
	return len(parts) == 2 && parts[1] == "activate"
}

func validateLifecycleBody(raw []byte) ([]byte, error) {
	if err := rejectNonEmptyBody(raw); err != nil {
		return nil, err
	}
	return nil, nil
}

func validateResolveBody(raw []byte, vc ratesrbac.VerifiedContext) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicResolveRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
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
	return marshalSanitized(req)
}

func validateCreateContractBody(raw []byte, vc ratesrbac.VerifiedContext) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicCreateContractRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	selectedCompany, err := uuid.Parse(vc.CompanyID)
	if err != nil {
		return nil, apperrors.Forbidden("verified company context is required")
	}
	if req.BuyerCompanyID != selectedCompany {
		return nil, apperrors.Forbidden("buyer_company_id must match selected company")
	}
	return marshalSanitized(req)
}

func validatePatchContractBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicPatchContractRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	if !patchContractHasChanges(req) {
		return nil, apperrors.Validation("request body must include at least one patch field", map[string]any{"field": "body"})
	}
	if len(req.ValidTo) > 0 {
		if err := validateNullableDateJSON(req.ValidTo, "valid_to"); err != nil {
			return nil, err
		}
	}
	return marshalSanitized(req)
}

func patchContractHasChanges(req publicPatchContractRequest) bool {
	return req.Name != nil || req.Description != nil || req.ExternalReference != nil || len(req.ValidTo) > 0
}

func validateCreateRateCardBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicCreateRateCardRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	return marshalSanitized(req)
}

func validateCreateRateVersionBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicCreateRateVersionRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.ValidFrom) == "" {
		return nil, apperrors.Validation("valid_from is required", map[string]any{"field": "valid_from"})
	}
	if err := validateDateStringJSON(json.RawMessage(`"`+strings.TrimSpace(req.ValidFrom)+`"`), "valid_from"); err != nil {
		return nil, err
	}
	if len(req.ValidTo) > 0 {
		if err := validateNullableDateJSON(req.ValidTo, "valid_to"); err != nil {
			return nil, err
		}
	}
	return marshalSanitized(req)
}

func validatePatchRateVersionBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicPatchRateVersionRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	if len(req.ValidFrom) == 0 && len(req.ValidTo) == 0 {
		return nil, apperrors.Validation("request body must include at least one patch field", map[string]any{"field": "body"})
	}
	if len(req.ValidFrom) > 0 {
		if err := validateDateStringJSON(req.ValidFrom, "valid_from"); err != nil {
			return nil, err
		}
	}
	if len(req.ValidTo) > 0 {
		if err := validateNullableDateJSON(req.ValidTo, "valid_to"); err != nil {
			return nil, err
		}
	}
	return marshalSanitized(req)
}

func validateCreateRateLineBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicCreateRateLineRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	return marshalSanitized(req)
}

func validatePatchRateLineBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicPatchRateLineRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	if req.OriginLocationID == nil && req.DestinationLocationID == nil && req.EquipmentType == nil && req.TransportMode == nil {
		return nil, apperrors.Validation("request body must include at least one patch field", map[string]any{"field": "body"})
	}
	return marshalSanitized(req)
}

func validateCreateRateComponentBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicCreateRateComponentRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	return marshalSanitized(req)
}

func validatePatchRateComponentBody(raw []byte) ([]byte, error) {
	if err := requireNonEmptyBody(raw); err != nil {
		return nil, err
	}
	var req publicPatchRateComponentRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	if req.Amount == nil && req.PercentValue == nil && req.UnitCode == nil {
		return nil, apperrors.Validation("request body must include at least one patch field", map[string]any{"field": "body"})
	}
	return marshalSanitized(req)
}

func validateTerminateBody(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var req publicTerminateRequest
	if err := decodeStrictJSON(raw, &req); err != nil {
		return nil, err
	}
	return marshalSanitized(req)
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
