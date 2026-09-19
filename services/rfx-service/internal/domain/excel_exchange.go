package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	WorkbookTypeBuyerTender  = "BUYER_TENDER"
	WorkbookTypeCarrierOffer = "CARRIER_OFFER"

	SchemaVersionBuyerXLSXV1   = "BINTRANS_RFX_BUYER_XLSX_V1"
	SchemaVersionCarrierXLSXV1 = "BINTRANS_RFX_CARRIER_XLSX_V1"

	ImportAnalysisStatusPreviewed = "PREVIEWED"
	ImportAnalysisStatusConsumed  = "CONSUMED"
	ImportAnalysisStatusExpired   = "EXPIRED"

	ImportTargetTypeNewEvent        = "NEW_EVENT"
	ImportTargetTypeDraftEvent      = "DRAFT_EVENT"
	ImportTargetTypeCarrierResponse = "CARRIER_RESPONSE"

	BuyerXlsxImportCommitOperation   = "BUYER_XLSX_IMPORT_COMMIT"
	CarrierXlsxImportCommitOperation = "CARRIER_XLSX_IMPORT_COMMIT"

	MachineCodeAnalysisExpired         = "analysis_expired"
	MachineCodeAnalysisAlreadyConsumed = "analysis_already_consumed"
	MachineCodeStaleTarget             = "stale_target"
	MachineCodeAnalysisNotFound        = "analysis_not_found"
	MachineCodeProposalRevalidation    = "proposal_revalidation_failed"
	MachineCodeCanonicalHashMismatch   = "canonical_hash_mismatch"
	MachineCodeActorBindingDenied      = "actor_binding_denied"
	MachineCodeIdempotencyConflict     = "idempotency_conflict"
	MachineCodeStaleMappingContext     = "stale_mapping_context"
	MachineCodeExternalIDConflict      = "external_id_conflict"

	CreationChannelManual   = "MANUAL"
	CreationChannelTemplate = "TEMPLATE"
	CreationChannelExcel    = "EXCEL"
	CreationChannelERP      = "ERP"
)

type ImportAnalysis struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	ActorID                uuid.UUID
	IntegrationPrincipalID *uuid.UUID
	ActorCompanyID         uuid.UUID
	WorkbookType           string
	SchemaVersion          string
	TargetType             string
	TargetID               *uuid.UUID
	TargetVersion          *int
	CanonicalPayloadJSON   []byte
	CanonicalHash          string
	Status                 string
	ValidationSummary      []byte
	ExpiresAt              time.Time
	ConsumedAt             *time.Time
	ResultReferenceType    *string
	ResultReferenceID      *uuid.UUID
	CreatedAt              time.Time
}

type BuyerImportCommitInput struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
}

// BuyerImportCommitIdempotencyPayload is the canonical request fingerprint for commit replay.
type BuyerImportCommitIdempotencyPayload struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
}

func NewBuyerImportCommitIdempotencyPayload(analysisID uuid.UUID) BuyerImportCommitIdempotencyPayload {
	return BuyerImportCommitIdempotencyPayload{AnalysisID: analysisID}
}

func ValidateBuyerImportCommitInput(in BuyerImportCommitInput) error {
	if in.AnalysisID == uuid.Nil {
		return apperrors.Validation("analysis_id is required", map[string]any{"field": "analysis_id"})
	}
	return nil
}

type ErpCreateCommitInput struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
}

type ErpCreateCommitIdempotencyPayload struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
}

func NewErpCreateCommitIdempotencyPayload(analysisID uuid.UUID) ErpCreateCommitIdempotencyPayload {
	return ErpCreateCommitIdempotencyPayload{AnalysisID: analysisID}
}

func ValidateErpCreateCommitInput(in ErpCreateCommitInput) error {
	if in.AnalysisID == uuid.Nil {
		return apperrors.Validation("analysis_id is required", map[string]any{"field": "analysis_id"})
	}
	return nil
}

type CarrierImportCommitInput struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
}

// CarrierImportCommitIdempotencyPayload is the canonical request fingerprint for carrier commit replay.
type CarrierImportCommitIdempotencyPayload struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
}

func NewCarrierImportCommitIdempotencyPayload(analysisID uuid.UUID) CarrierImportCommitIdempotencyPayload {
	return CarrierImportCommitIdempotencyPayload{AnalysisID: analysisID}
}

func ValidateCarrierImportCommitInput(in CarrierImportCommitInput) error {
	if in.AnalysisID == uuid.Nil {
		return apperrors.Validation("analysis_id is required", map[string]any{"field": "analysis_id"})
	}
	return nil
}

type ExternalObjectLink struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	IntegrationPrincipalID uuid.UUID
	ExternalSystem         string
	ExternalObjectType     string
	ExternalObjectID       string
	ExternalVersion        string
	ExternalRevision       string
	PayloadHash            string
	RfxEventID             uuid.UUID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type ExternalObjectLinkRevision struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	LinkID           uuid.UUID
	ExternalRevision string
	PayloadHash      string
	RecordedAt       time.Time
}

func ValidateCreationChannel(value string) error {
	switch strings.TrimSpace(value) {
	case CreationChannelManual, CreationChannelTemplate, CreationChannelExcel, CreationChannelERP:
		return nil
	default:
		return apperrors.Validation("invalid creation_channel", map[string]any{"field": "creation_channel", "value": value})
	}
}

func ValidateImportAnalysisPreviewInput(in ImportAnalysis) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if err := ValidateImportAnalysisOwner(in.ActorID, in.IntegrationPrincipalID); err != nil {
		return err
	}
	if in.ActorID == uuid.Nil {
		return apperrors.Validation("actor_id is required", map[string]any{"field": "actor_id"})
	}
	if in.ActorCompanyID == uuid.Nil {
		return apperrors.Validation("actor_company_id is required", map[string]any{"field": "actor_company_id"})
	}
	if err := validateWorkbookType(in.WorkbookType); err != nil {
		return err
	}
	if err := validateSchemaVersion(in.WorkbookType, in.SchemaVersion); err != nil {
		return err
	}
	if err := validateImportTargetType(in.TargetType); err != nil {
		return err
	}
	if len(in.CanonicalPayloadJSON) == 0 {
		return apperrors.Validation("canonical_payload_json is required", map[string]any{"field": "canonical_payload_json"})
	}
	if len(strings.TrimSpace(in.CanonicalHash)) != 64 {
		return apperrors.Validation("canonical_hash must be sha256 hex", map[string]any{"field": "canonical_hash"})
	}
	if err := VerifyImportAnalysisCanonicalHash(in.CanonicalPayloadJSON, in.CanonicalHash); err != nil {
		return err
	}
	if in.ExpiresAt.IsZero() {
		return apperrors.Validation("expires_at is required", map[string]any{"field": "expires_at"})
	}
	if !in.CreatedAt.IsZero() && !in.ExpiresAt.After(in.CreatedAt) {
		return apperrors.Validation("expires_at must be after created_at", map[string]any{"field": "expires_at"})
	}
	return nil
}

// VerifyImportAnalysisCanonicalHash ensures persisted hash matches canonical payload bytes.
func VerifyImportAnalysisCanonicalHash(payloadJSON []byte, hash string) error {
	sum := sha256.Sum256(payloadJSON)
	computed := hex.EncodeToString(sum[:])
	if strings.ToLower(strings.TrimSpace(hash)) != computed {
		return apperrors.Validation("canonical hash mismatch", map[string]any{"field": "canonical_hash"})
	}
	return nil
}

func validateWorkbookType(value string) error {
	switch strings.TrimSpace(value) {
	case WorkbookTypeBuyerTender, WorkbookTypeCarrierOffer:
		return nil
	default:
		return apperrors.Validation("invalid workbook_type", map[string]any{"field": "workbook_type", "value": value})
	}
}

func validateSchemaVersion(workbookType, schemaVersion string) error {
	schemaVersion = strings.TrimSpace(schemaVersion)
	switch strings.TrimSpace(workbookType) {
	case WorkbookTypeBuyerTender:
		if schemaVersion != SchemaVersionBuyerXLSXV1 {
			return apperrors.Validation("unsupported buyer schema_version", map[string]any{"field": "schema_version", "value": schemaVersion})
		}
	case WorkbookTypeCarrierOffer:
		if schemaVersion != SchemaVersionCarrierXLSXV1 {
			return apperrors.Validation("unsupported carrier schema_version", map[string]any{"field": "schema_version", "value": schemaVersion})
		}
	}
	return nil
}

func validateImportTargetType(value string) error {
	switch strings.TrimSpace(value) {
	case ImportTargetTypeNewEvent, ImportTargetTypeDraftEvent, ImportTargetTypeCarrierResponse:
		return nil
	default:
		return apperrors.Validation("invalid target_type", map[string]any{"field": "target_type", "value": value})
	}
}
