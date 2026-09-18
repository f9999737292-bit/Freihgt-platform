package erpjson

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

var failClosedMappingTypes = map[string]struct{}{
	"CURRENCY":     {},
	"COUNTRY":      {},
	"UNIT":         {},
	"TIMEZONE":     {},
	"CARRIER_CODE": {},
}

var warningMappingTypes = map[string]struct{}{
	"CARGO_TYPE":   {},
	"VEHICLE_BODY": {},
}

type freightExtensions struct {
	UnitCode  string `json:"unit_code"`
	CargoType string `json:"cargo_type"`
}

func parseFreightExtensions(raw json.RawMessage) freightExtensions {
	if len(raw) == 0 {
		return freightExtensions{}
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return freightExtensions{}
	}
	freightRaw, ok := root["freight"]
	if !ok {
		return freightExtensions{}
	}
	var out freightExtensions
	_ = json.Unmarshal(freightRaw, &out)
	return out
}

func resolveMappedField(
	ctx context.Context,
	tenantID uuid.UUID,
	maps MappingLookup,
	mappingType string,
	path string,
	externalCode string,
) (Issue, *domain.ReferenceMappingSet, bool) {
	externalCode = strings.TrimSpace(externalCode)
	if externalCode == "" {
		return Issue{}, nil, false
	}
	_, set, _, err := maps.ResolveCode(ctx, tenantID, mappingType, externalCode)
	if err == nil {
		return Issue{}, &set, true
	}
	issue := mappingIssueFromErr(err, mappingType, path, externalCode)
	_, warnOnly := warningMappingTypes[mappingType]
	return issue, nil, warnOnly
}

func mappingIssueFromErr(err error, mappingType, path, externalSource string) Issue {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Code == apperrors.CodeValidation && appErr.Details != nil {
		if code, ok := appErr.Details["machine_code"].(string); ok {
			switch code {
			case MachineCodeMappingRetired:
				return Issue{
					Severity: SeverityError, MachineCode: MachineCodeMappingRetired,
					Path: path, MessageKey: "rfx.erp.mapping_retired",
				}
			case MachineCodeMappingAmbiguous:
				return Issue{
					Severity: SeverityError, MachineCode: MachineCodeMappingAmbiguous,
					Path: path, MessageKey: "rfx.erp.mapping_ambiguous",
				}
			}
		}
	}
	return Issue{
		Severity:       SeverityError,
		MachineCode:    MachineCodeMappingNotFound,
		Path:           path,
		MessageKey:     "rfx.erp.mapping_not_found",
		ExternalSource: externalSource,
		Params:         map[string]any{"mapping_type": mappingType},
	}
}
