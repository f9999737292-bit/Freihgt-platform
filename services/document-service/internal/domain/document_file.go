package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

const (
	FileTypePDF       = "PDF"
	FileTypeXML       = "XML"
	FileTypeJSON      = "JSON"
	FileTypeImage     = "IMAGE"
	FileTypeSignature = "SIGNATURE"
	FileTypeOther     = "OTHER"
)

var allowedFileTypes = map[string]struct{}{
	FileTypePDF: {}, FileTypeXML: {}, FileTypeJSON: {},
	FileTypeImage: {}, FileTypeSignature: {}, FileTypeOther: {},
}

type DocumentFile struct {
	ID                uuid.UUID
	DocumentID        uuid.UUID
	DocumentVersionID *uuid.UUID
	FileType          string
	StorageProvider   string
	BucketName        *string
	ObjectKey         string
	FileName          *string
	MimeType          *string
	FileSizeBytes     *int64
	ChecksumSHA256    *string
	CreatedAt         time.Time
}

type CreateDocumentFileInput struct {
	TenantID          uuid.UUID
	DocumentVersionID uuid.UUID
	FileType          string
	StorageProvider   string
	BucketName        *string
	ObjectKey         string
	FileName          *string
	MimeType          *string
	FileSizeBytes     *int64
	ChecksumSHA256    *string
}

func ValidateCreateDocumentFileInput(in CreateDocumentFileInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.DocumentVersionID == uuid.Nil {
		return apperrors.Validation("document_version_id is required", map[string]any{"field": "document_version_id"})
	}
	if err := ValidateFileType(in.FileType); err != nil {
		return err
	}
	if strings.TrimSpace(in.ObjectKey) == "" {
		return apperrors.Validation("object_key is required", map[string]any{"field": "object_key"})
	}
	if strings.TrimSpace(in.StorageProvider) == "" {
		in.StorageProvider = "S3"
	}
	return nil
}

func ValidateFileType(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return apperrors.Validation("file_type is required", map[string]any{"field": "file_type"})
	}
	if _, ok := allowedFileTypes[value]; !ok {
		return apperrors.Validation("invalid file_type", map[string]any{"field": "file_type", "value": value})
	}
	return nil
}

func NormalizeStorageProvider(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "S3"
	}
	return value
}
