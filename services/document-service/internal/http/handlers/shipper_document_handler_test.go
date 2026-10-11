package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
)

type shipperHTTPFake struct {
	doc *domain.Document
	att *domain.Attachment
}

func (f shipperHTTPFake) List(context.Context, domain.ShipperDocumentFilter) ([]domain.Document, int, error) {
	return []domain.Document{*f.doc}, 1, nil
}
func (f shipperHTTPFake) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Document, error) {
	return f.doc, nil
}
func (f shipperHTTPFake) ListAttachments(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) ([]domain.Attachment, error) {
	return []domain.Attachment{*f.att}, nil
}
func (f shipperHTTPFake) GetAttachment(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, error) {
	return f.att, nil
}
func (f shipperHTTPFake) OpenContent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	return f.att, io.NopCloser(strings.NewReader("%PDF")), nil
}

func TestShipperDocumentJSONOmitsStorageMetadata(t *testing.T) {
	now := time.Now().UTC()
	shipment := uuid.New()
	rel := domain.RelatedEntityTypeShipment
	doc := &domain.Document{
		ID: uuid.New(), TenantID: uuid.New(), DocumentNumber: "POD-1", DocumentType: "POD",
		DocumentStatus: domain.DocumentStatusSigned, OwnerCompanyID: uuid.New(),
		RelatedEntityType: &rel, RelatedEntityID: &shipment, LegalLanguage: "ru-RU",
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	key := "tenants/secret/object"
	idem := "idem-secret"
	att := &domain.Attachment{
		ID: uuid.New(), DocumentID: doc.ID, TenantID: doc.TenantID, FileName: "pod.pdf",
		MediaType: "application/pdf", SizeBytes: 4, SHA256: strings.Repeat("ab", 32),
		StorageKey: key, Status: domain.AttachmentStatusFinalized, IdempotencyKey: &idem,
		CreatedAt: now, FinalizedAt: &now,
	}
	handler := NewShipperDocumentHandler(shipperHTTPFake{doc: doc, att: att})
	router := chi.NewRouter()
	router.Get("/v1/shipper/documents", handler.List)
	router.Get("/v1/shipper/documents/{documentId}", handler.Get)
	router.Get("/v1/shipper/documents/{documentId}/attachments", handler.ListAttachments)
	router.Get("/v1/shipper/documents/{documentId}/attachments/{attachmentId}", handler.GetAttachment)
	router.Get("/v1/shipper/documents/{documentId}/attachments/{attachmentId}/content", handler.Content)

	shipper := uuid.New()
	paths := []string{
		"/v1/shipper/documents?shipper_company_id=" + shipper.String() + "&shipment_id=" + shipment.String(),
		"/v1/shipper/documents/" + doc.ID.String() + "?shipper_company_id=" + shipper.String(),
		"/v1/shipper/documents/" + doc.ID.String() + "/attachments?shipper_company_id=" + shipper.String(),
		"/v1/shipper/documents/" + doc.ID.String() + "/attachments/" + att.ID.String() + "?shipper_company_id=" + shipper.String(),
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Tenant-ID", doc.TenantID.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
		assertNoStorageLeak(t, rec.Body.String())
	}
	contentReq := httptest.NewRequest(http.MethodGet, "/v1/shipper/documents/"+doc.ID.String()+"/attachments/"+att.ID.String()+"/content?shipper_company_id="+shipper.String(), nil)
	contentReq.Header.Set("X-Tenant-ID", doc.TenantID.String())
	contentRec := httptest.NewRecorder()
	router.ServeHTTP(contentRec, contentReq)
	if contentRec.Code != http.StatusOK || contentRec.Header().Get("X-Content-SHA256") != att.SHA256 {
		t.Fatalf("content status=%d", contentRec.Code)
	}
	if contentRec.Header().Get("Content-Disposition") != `attachment; filename="pod.pdf"` {
		t.Fatalf("disposition=%s", contentRec.Header().Get("Content-Disposition"))
	}
	joined := contentRec.Header().Get("Content-Disposition") + contentRec.Header().Get("Content-Type") + contentRec.Header().Get("X-Content-SHA256")
	if strings.Contains(joined, key) || strings.Contains(joined, "storage") {
		t.Fatal("content headers disclose storage")
	}
}

func TestShipperListRequiresShipmentID(t *testing.T) {
	handler := NewShipperDocumentHandler(shipperHTTPFake{})
	router := chi.NewRouter()
	router.Get("/v1/shipper/documents", handler.List)
	req := httptest.NewRequest(http.MethodGet, "/v1/shipper/documents?shipper_company_id="+uuid.New().String(), nil)
	req.Header.Set("X-Tenant-ID", uuid.New().String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	bad := httptest.NewRequest(http.MethodGet, "/v1/shipper/documents?shipper_company_id="+uuid.New().String()+"&shipment_id=nope", nil)
	bad.Header.Set("X-Tenant-ID", uuid.New().String())
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("malformed status=%d", badRec.Code)
	}
}

func assertNoStorageLeak(t *testing.T, body string) {
	t.Helper()
	for _, token := range []string{
		"storageKey", "storage_key", "bucketName", "bucket_name", "storageProvider", "storage_provider",
		"objectKey", "object_key", "payloadXmlPath", "payload_xml_path", "pdfFilePath", "pdf_file_path",
		"idempotencyKey", "idempotency_key", "payload_json", "payloadJson",
	} {
		if strings.Contains(body, token) {
			t.Fatalf("response contains %s", token)
		}
	}
}
