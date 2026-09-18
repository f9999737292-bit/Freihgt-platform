package erpjson

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

const (
	OperationCreateDraft = "CREATE_DRAFT"
	OperationUpdateDraft = "UPDATE_DRAFT"
)

type RawDocument struct {
	SchemaVersion       string          `json:"schema_version"`
	RequestedOperation  string          `json:"requested_operation"`
	External            *ExternalRef    `json:"external,omitempty"`
	Event               EventPayload    `json:"event"`
	Lots                []LotPayload    `json:"lots,omitempty"`
	Questionnaire       json.RawMessage `json:"questionnaire,omitempty"`
	TemplateReference   json.RawMessage `json:"template_reference,omitempty"`
	Extensions          json.RawMessage `json:"extensions,omitempty"`
	MappingContext      json.RawMessage `json:"mapping_context,omitempty"`
}

type ExternalRef struct {
	System   string `json:"system"`
	ObjectID string `json:"object_id"`
	Revision string `json:"revision,omitempty"`
}

type EventPayload struct {
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Currency    string     `json:"currency"`
	Timezone    string     `json:"timezone"`
	Deadline    *time.Time `json:"deadline,omitempty"`
}

type LotPayload struct {
	LotNumber      string   `json:"lot_number"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Category       string   `json:"category,omitempty"`
	EstimatedValue *float64 `json:"estimated_value,omitempty"`
	CurrencyCode   string   `json:"currency_code,omitempty"`
}

type MappingContextPin struct {
	MappingSetID      uuid.UUID `json:"mapping_set_id"`
	MappingSetVersion int       `json:"mapping_set_version"`
	MappingTypes      []string  `json:"mapping_types_applied"`
}

type ParsedPreview struct {
	Operation       string
	External        *ExternalRef
	Event           EventPayload
	Lots            []domain.CreateRfxLotInput
	Questionnaire   domain.QuestionnaireDefinition
	ReadyToCommit   bool
	Errors          []Issue
	Warnings        []Issue
	MappingContext  MappingContextPin
	CanonicalJSON   []byte
	CanonicalHash   string
}

type PreviewResponse struct {
	SchemaVersion       string     `json:"schema_version"`
	ReadyToCommit       bool       `json:"ready_to_commit"`
	AnalysisID          *uuid.UUID `json:"analysis_id,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
	CanonicalPayloadHash string    `json:"canonical_payload_hash,omitempty"`
	Errors              []Issue    `json:"errors"`
	Warnings            []Issue    `json:"warnings"`
}
