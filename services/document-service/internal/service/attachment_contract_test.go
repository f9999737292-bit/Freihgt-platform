package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/freight-platform/document-service/internal/domain"
)

func TestAttachmentOpenAPIDoesNotExposeStorageSecrets(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "packages", "openapi", "document-service.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(raw)
	for _, path := range []string{
		"/api/v1/documents/{id}/attachments:",
		"/api/v1/documents/{id}/attachments/{attachmentId}/content:",
		"operationId: post_document_attachment",
		"operationId: get_document_attachment_content",
	} {
		if !strings.Contains(spec, path) {
			t.Fatalf("openapi missing %s", path)
		}
	}
	for _, secret := range []string{"EDO_OBJECT_STORAGE_SECRET_KEY", "access_key_id", "storage_key"} {
		if strings.Contains(spec, secret) {
			t.Fatalf("openapi exposes %s", secret)
		}
	}
	if domain.MaxAttachmentBytes != 10<<20 {
		t.Fatalf("upload limit changed: %d", domain.MaxAttachmentBytes)
	}
}
