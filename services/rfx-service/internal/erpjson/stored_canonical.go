package erpjson

import (
	"encoding/json"
	"strings"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type StoredCreateCanonical struct {
	SchemaVersion      string            `json:"schema_version"`
	RequestedOperation string            `json:"requested_operation"`
	External           *ExternalRef      `json:"external,omitempty"`
	Event              EventPayload      `json:"event"`
	Lots               []LotPayload      `json:"lots,omitempty"`
	MappingContext     MappingContextPin `json:"mapping_context"`
}

type UpdateBaselineTokens struct {
	EventRowVersion         int    `json:"event_row_version"`
	DraftRowVersion         int    `json:"draft_row_version"`
	BaselineLotsFingerprint string `json:"baseline_lots_fingerprint"`
}

type StoredUpdateCanonical struct {
	SchemaVersion           string              `json:"schema_version"`
	RequestedOperation      string              `json:"requested_operation"`
	External                *ExternalRef        `json:"external,omitempty"`
	Event                   EventPayload        `json:"event"`
	Lots                    []LotPayload        `json:"lots,omitempty"`
	Questionnaire           json.RawMessage     `json:"questionnaire,omitempty"`
	MappingContext          MappingContextPin   `json:"mapping_context"`
	EventRowVersion         int                 `json:"event_row_version"`
	DraftRowVersion         int                 `json:"draft_row_version"`
	BaselineLotsFingerprint string              `json:"baseline_lots_fingerprint"`
}

func BindUpdateBaselineTokens(canonical []byte, tokens UpdateBaselineTokens) ([]byte, error) {
	if tokens.EventRowVersion <= 0 || tokens.DraftRowVersion <= 0 || strings.TrimSpace(tokens.BaselineLotsFingerprint) == "" {
		return nil, apperrors.Internal("update baseline tokens are incomplete", nil)
	}
	var doc map[string]any
	if err := json.Unmarshal(canonical, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	doc["event_row_version"] = tokens.EventRowVersion
	doc["draft_row_version"] = tokens.DraftRowVersion
	doc["baseline_lots_fingerprint"] = tokens.BaselineLotsFingerprint
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return NormalizeJSON(raw)
}

func ParseStoredUpdateCanonical(raw []byte) (*StoredUpdateCanonical, error) {
	if len(raw) == 0 {
		return nil, apperrors.Unprocessable("stored import analysis payload invalid", map[string]any{
			"machine_code": domain.MachineCodeCanonicalHashMismatch,
		})
	}
	var stored StoredUpdateCanonical
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, apperrors.Unprocessable("stored import analysis payload invalid", map[string]any{
			"machine_code": domain.MachineCodeCanonicalHashMismatch,
		})
	}
	if stored.External != nil {
		stored.External.System = strings.ToUpper(strings.TrimSpace(stored.External.System))
		stored.External.ObjectID = strings.TrimSpace(stored.External.ObjectID)
		stored.External.Revision = strings.TrimSpace(stored.External.Revision)
	}
	if stored.EventRowVersion <= 0 || stored.DraftRowVersion <= 0 || strings.TrimSpace(stored.BaselineLotsFingerprint) == "" {
		return nil, apperrors.Unprocessable("stored import analysis baseline tokens missing", map[string]any{
			"machine_code": domain.MachineCodeProposalRevalidation,
		})
	}
	return &stored, nil
}

func ParseStoredCreateCanonical(raw []byte) (*StoredCreateCanonical, error) {
	if len(raw) == 0 {
		return nil, apperrors.Unprocessable("stored import analysis payload invalid", map[string]any{
			"machine_code": domain.MachineCodeCanonicalHashMismatch,
		})
	}
	var stored StoredCreateCanonical
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, apperrors.Unprocessable("stored import analysis payload invalid", map[string]any{
			"machine_code": domain.MachineCodeCanonicalHashMismatch,
		})
	}
	if stored.External != nil {
		stored.External.System = strings.ToUpper(strings.TrimSpace(stored.External.System))
		stored.External.ObjectID = strings.TrimSpace(stored.External.ObjectID)
		stored.External.Revision = strings.TrimSpace(stored.External.Revision)
	}
	return &stored, nil
}
