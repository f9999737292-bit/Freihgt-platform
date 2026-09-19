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
