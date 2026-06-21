package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type DocumentVersion struct {
	ID             uuid.UUID
	DocumentID     uuid.UUID
	VersionNumber  int
	PayloadJSON    json.RawMessage
	PayloadXMLPath *string
	PDFFilePath    *string
	CreatedAt      time.Time
}

type CreateDocumentVersionInput struct {
	TenantID       uuid.UUID
	PayloadJSON    json.RawMessage
	PayloadXMLPath *string
	PDFFilePath    *string
}

func ValidateCreateDocumentVersionInput(in CreateDocumentVersionInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	return nil
}
