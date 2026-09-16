package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	WorkbookTypeERPBuyerJSON = "ERP_BUYER_JSON"
	SchemaVersionERPJSONV1   = "BINTRANS_RFX_ERP_JSON_V1"

	IntegrationCredentialTypeOAuth  = "OAUTH"
	IntegrationCredentialTypeAPIKey = "API_KEY"

	IntegrationPrincipalStatusActive    = "ACTIVE"
	IntegrationPrincipalStatusRevoked   = "REVOKED"
	IntegrationPrincipalStatusSuspended = "SUSPENDED"

	ReferenceMappingSetStatusActive  = "ACTIVE"
	ReferenceMappingSetStatusRetired = "RETIRED"

	ScopeDraftCreate  = "rfx:draft:create"
	ScopeDraftRead    = "rfx:draft:read"
	ScopeDraftPreview = "rfx:draft:preview"
	ScopeDraftCommit  = "rfx:draft:commit"
	ScopeStatusRead   = "rfx:status:read"

	ERPBuyerCreatePreviewOperation = "ERP_BUYER_CREATE_PREVIEW"
	ERPBuyerCreateCommitOperation  = "ERP_BUYER_CREATE_COMMIT"
	ERPBuyerUpdatePreviewOperation = "ERP_BUYER_UPDATE_PREVIEW"
	ERPBuyerUpdateCommitOperation  = "ERP_BUYER_UPDATE_COMMIT"

	OwnerKindHuman                = "HUMAN"
	OwnerKindIntegrationPrincipal = "INTEGRATION_PRINCIPAL"
)

type IntegrationPrincipal struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	CompanyID      uuid.UUID
	ClientID       string
	CredentialType string
	Status         string
	AllowedCIDRs   []byte
	RevokedAt      *time.Time
	GraceEndsAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type IntegrationCredential struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	IntegrationPrincipalID uuid.UUID
	CredentialType         string
	SecretHash             string
	HashAlgorithm          string
	HashVersion            int
	LookupFingerprint      string
}

type ReferenceMappingSet struct {
	ID          uuid.UUID
	TenantID    *uuid.UUID
	MappingType string
	Version     int
	Status      string
}

type ReferenceMappingEntry struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	MappingSetID  uuid.UUID
	ExternalCode  string
	CanonicalCode string
}

func NormalizeIntegrationClientID(clientID string) string {
	return strings.ToLower(strings.TrimSpace(clientID))
}

func ValidateImportAnalysisOwner(actorID uuid.UUID, integrationPrincipalID *uuid.UUID) error {
	hasActor := actorID != uuid.Nil
	hasPrincipal := integrationPrincipalID != nil && *integrationPrincipalID != uuid.Nil
	switch {
	case hasActor && hasPrincipal:
		return apperrors.Validation("actor_id and integration_principal_id are mutually exclusive", map[string]any{
			"field": "owner",
		})
	case !hasActor && !hasPrincipal:
		return apperrors.Validation("exactly one owner is required", map[string]any{
			"field": "owner",
		})
	default:
		return nil
	}
}

func ValidateHumanImportAnalysisPreviewInput(in ImportAnalysis) error {
	if in.IntegrationPrincipalID != nil && *in.IntegrationPrincipalID != uuid.Nil {
		return apperrors.Validation("integration_principal_id must be null for human import preview", map[string]any{
			"field": "integration_principal_id",
		})
	}
	return ValidateImportAnalysisPreviewInput(in)
}

func ValidateERPImportAnalysisPreviewInput(in ImportAnalysis) error {
	if in.ActorID != uuid.Nil {
		return apperrors.Validation("actor_id must be null for ERP import preview", map[string]any{
			"field": "actor_id",
		})
	}
	if in.IntegrationPrincipalID == nil || *in.IntegrationPrincipalID == uuid.Nil {
		return apperrors.Validation("integration_principal_id is required", map[string]any{
			"field": "integration_principal_id",
		})
	}
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.ActorCompanyID == uuid.Nil {
		return apperrors.Validation("actor_company_id is required", map[string]any{"field": "actor_company_id"})
	}
	if strings.TrimSpace(in.WorkbookType) != WorkbookTypeERPBuyerJSON {
		return apperrors.Validation("invalid workbook_type for ERP import preview", map[string]any{
			"field": "workbook_type", "value": in.WorkbookType,
		})
	}
	if strings.TrimSpace(in.SchemaVersion) != SchemaVersionERPJSONV1 {
		return apperrors.Validation("unsupported ERP schema_version", map[string]any{
			"field": "schema_version", "value": in.SchemaVersion,
		})
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
	return nil
}

func ValidateIntegrationScope(scope string) error {
	switch strings.TrimSpace(scope) {
	case ScopeDraftCreate, ScopeDraftRead, ScopeDraftPreview, ScopeDraftCommit, ScopeStatusRead:
		return nil
	default:
		return apperrors.Validation("unsupported integration scope", map[string]any{"field": "scope", "value": scope})
	}
}
