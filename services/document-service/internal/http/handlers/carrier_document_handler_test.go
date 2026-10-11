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
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/service"
)

type carrierHTTPFake struct {
	calls   int
	tenant  uuid.UUID
	carrier uuid.UUID
	doc     *domain.Document
	att     *domain.Attachment
}

func (f *carrierHTTPFake) allow(tenantID, carrierID uuid.UUID) bool {
	return tenantID == f.tenant && carrierID == f.carrier
}

func (f *carrierHTTPFake) List(_ context.Context, filter domain.CarrierDocumentFilter) ([]domain.Document, int, error) {
	f.calls++
	if !f.allow(filter.TenantID, filter.CarrierCompanyID) {
		return nil, 0, apperrors.NotFound("shipment not found")
	}
	return []domain.Document{*f.doc}, 1, nil
}

func (f *carrierHTTPFake) Get(_ context.Context, tenantID, carrierID, _ uuid.UUID) (*domain.Document, error) {
	f.calls++
	if !f.allow(tenantID, carrierID) {
		return nil, apperrors.NotFound("document not found")
	}
	return f.doc, nil
}

func (f *carrierHTTPFake) ListAttachments(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) ([]domain.Attachment, error) {
	f.calls++
	return []domain.Attachment{*f.att}, nil
}

func (f *carrierHTTPFake) GetAttachment(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, error) {
	f.calls++
	return f.att, nil
}

func (f *carrierHTTPFake) OpenContent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	f.calls++
	return f.att, io.NopCloser(strings.NewReader("%PDF")), nil
}

func TestCarrierDocumentRoutes(t *testing.T) {
	now := time.Now().UTC()
	tenant := uuid.New()
	carrier := uuid.New()
	shipper := uuid.New()
	consignee := uuid.New()
	forwarder := uuid.New()
	otherCarrier := uuid.New()
	shipment := uuid.New()
	rel := domain.RelatedEntityTypeShipment
	doc := &domain.Document{
		ID: uuid.New(), TenantID: tenant, DocumentNumber: "POD-1", DocumentType: "POD",
		DocumentStatus: domain.DocumentStatusSigned, OwnerCompanyID: shipper,
		RelatedEntityType: &rel, RelatedEntityID: &shipment, LegalLanguage: "ru-RU",
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	key := "tenants/secret/object"
	idem := "idem-secret"
	att := &domain.Attachment{
		ID: uuid.New(), DocumentID: doc.ID, TenantID: tenant, FileName: "pod.pdf",
		MediaType: "application/pdf", SizeBytes: 4, SHA256: strings.Repeat("ab", 32),
		StorageKey: key, Status: domain.AttachmentStatusFinalized, IdempotencyKey: &idem,
		CreatedAt: now, FinalizedAt: &now,
	}
	fake := &carrierHTTPFake{tenant: tenant, carrier: carrier, doc: doc, att: att}
	handler := NewCarrierDocumentHandler(fake)
	router := chi.NewRouter()
	router.Get("/v1/carrier/documents", handler.List)
	router.Get("/v1/carrier/documents/{documentId}", handler.Get)
	router.Get("/v1/carrier/documents/{documentId}/attachments", handler.ListAttachments)
	router.Get("/v1/carrier/documents/{documentId}/attachments/{attachmentId}", handler.GetAttachment)
	router.Get("/v1/carrier/documents/{documentId}/attachments/{attachmentId}/content", handler.Content)

	get := func(path, tenantHeader string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if tenantHeader != "" {
			req.Header.Set("X-Tenant-ID", tenantHeader)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	list := func(company, shipmentID, extra string) string {
		return "/v1/carrier/documents?carrier_company_id=" + company + "&shipment_id=" + shipmentID + extra
	}

	ok := get(list(carrier.String(), shipment.String(), ""), tenant.String())
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "POD-1") {
		t.Fatalf("list status=%d body=%s", ok.Code, ok.Body.String())
	}
	assertNoStorageLeak(t, ok.Body.String())

	for _, company := range []uuid.UUID{otherCarrier, shipper, consignee, forwarder} {
		rec := get(list(company.String(), shipment.String(), ""), tenant.String())
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "POD-1") {
			t.Fatalf("alias %s status=%d body=%s", company, rec.Code, rec.Body.String())
		}
	}
	if rec := get(list(carrier.String(), shipment.String(), ""), uuid.New().String()); rec.Code != http.StatusNotFound {
		t.Fatalf("wrong tenant status=%d", rec.Code)
	}
	if rec := get("/v1/carrier/documents/"+doc.ID.String()+"?carrier_company_id="+otherCarrier.String(), tenant.String()); rec.Code != http.StatusNotFound {
		t.Fatalf("wrong carrier detail status=%d", rec.Code)
	}
	if rec := get(list("", shipment.String(), ""), tenant.String()); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing carrier status=%d", rec.Code)
	}
	if rec := get(list("nope", shipment.String(), ""), tenant.String()); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed carrier status=%d", rec.Code)
	}
	if rec := get("/v1/carrier/documents?carrier_company_id="+carrier.String(), tenant.String()); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing shipment status=%d", rec.Code)
	}
	if rec := get(list(carrier.String(), "nope", ""), tenant.String()); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed shipment status=%d", rec.Code)
	}
	before := fake.calls
	if rec := get(list(carrier.String(), shipment.String(), "&tenant_id="+uuid.New().String()), tenant.String()); rec.Code != http.StatusUnauthorized || fake.calls != before {
		t.Fatalf("conflict status=%d calls=%d", rec.Code, fake.calls)
	}
	if rec := get(list(carrier.String(), shipment.String(), "&related_entity_type=INVOICE"), tenant.String()); rec.Code != http.StatusBadRequest || fake.calls != before {
		t.Fatalf("override status=%d calls=%d", rec.Code, fake.calls)
	}

	paths := []string{
		"/v1/carrier/documents/" + doc.ID.String() + "?carrier_company_id=" + carrier.String(),
		"/v1/carrier/documents/" + doc.ID.String() + "/attachments?carrier_company_id=" + carrier.String(),
		"/v1/carrier/documents/" + doc.ID.String() + "/attachments/" + att.ID.String() + "?carrier_company_id=" + carrier.String(),
	}
	for _, path := range paths {
		rec := get(path, tenant.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
		assertNoStorageLeak(t, rec.Body.String())
	}
	content := get("/v1/carrier/documents/"+doc.ID.String()+"/attachments/"+att.ID.String()+"/content?carrier_company_id="+carrier.String(), tenant.String())
	if content.Code != http.StatusOK || content.Header().Get("Content-Type") != "application/pdf" || content.Header().Get("X-Content-SHA256") != att.SHA256 {
		t.Fatalf("content status=%d type=%s sha=%s", content.Code, content.Header().Get("Content-Type"), content.Header().Get("X-Content-SHA256"))
	}
	if content.Header().Get("Content-Disposition") != `attachment; filename="pod.pdf"` || strings.Contains(content.Header().Get("Content-Disposition"), key) {
		t.Fatalf("disposition=%s", content.Header().Get("Content-Disposition"))
	}
}

func TestCarrierListValidationDoesNotQuery(t *testing.T) {
	docs := &carrierQueryPanic{}
	handler := NewCarrierDocumentHandler(service.NewCarrierDocumentService(docs, docs))
	router := chi.NewRouter()
	router.Get("/v1/carrier/documents", handler.List)
	tenant := uuid.New().String()
	carrier := uuid.New().String()
	shipment := uuid.New().String()
	for _, path := range []string{
		"/v1/carrier/documents?carrier_company_id=" + carrier + "&shipment_id=" + shipment + "&limit=51",
		"/v1/carrier/documents?carrier_company_id=" + carrier + "&shipment_id=" + shipment + "&offset=-1",
		"/v1/carrier/documents?carrier_company_id=" + carrier + "&shipment_id=" + shipment + "&document_type=INVOICE",
		"/v1/carrier/documents?carrier_company_id=" + carrier + "&shipment_id=" + shipment + "&document_status=DRAFT",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Tenant-ID", tenant)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

type carrierQueryPanic struct{}

func (carrierQueryPanic) CarrierShipmentVisible(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	panic("shipment lookup before validation")
}
func (carrierQueryPanic) ListCarrierDocuments(context.Context, domain.CarrierDocumentFilter) ([]domain.Document, int, error) {
	panic("document query before validation")
}
func (carrierQueryPanic) GetCarrierDocument(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Document, error) {
	panic("document lookup")
}
func (carrierQueryPanic) ListFinalized(context.Context, uuid.UUID, uuid.UUID) ([]domain.Attachment, error) {
	panic("attachment list")
}
func (carrierQueryPanic) GetFinalized(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, error) {
	panic("attachment get")
}
func (carrierQueryPanic) OpenFinalized(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	panic("storage open")
}
