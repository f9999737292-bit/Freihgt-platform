package handlers

import (
	"bytes"
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
	"github.com/freight-platform/document-service/internal/service"
)

type fakeAttachments struct {
	detached int
	json     int
	status   *string
}

func (f *fakeAttachments) Create(context.Context, uuid.UUID, service.CreateAttachmentInput) (*domain.Attachment, error) {
	return nil, nil
}
func (f *fakeAttachments) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, error) {
	return nil, nil
}
func (f *fakeAttachments) Open(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, io.ReadCloser, error) {
	return nil, nil, nil
}
func (f *fakeAttachments) Finalize(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *uuid.UUID) (*domain.Attachment, error) {
	return nil, nil
}
func (f *fakeAttachments) AttachSignature(_ context.Context, _, _, _ uuid.UUID, in service.AttachSignatureInput) (*domain.AttachmentSignature, error) {
	f.json++
	return &domain.AttachmentSignature{ID: uuid.New(), SignatureFormat: in.Format, SignatureReference: in.Reference, VerificationStatus: domain.VerificationUnverified, EffectiveStatus: domain.VerificationUnverified, CreatedAt: time.Now()}, nil
}
func (f *fakeAttachments) AttachDetachedSignature(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, service.DetachedSignatureInput) (*domain.AttachmentSignature, error) {
	f.detached++
	return &domain.AttachmentSignature{ID: uuid.New(), SignatureReference: "bintrans:attachment-signature:test", VerificationStatus: domain.VerificationUnverified, EffectiveStatus: domain.VerificationPending, ReasonCode: domain.ReasonVerifierUnavailable, CreatedAt: time.Now()}, nil
}
func (f *fakeAttachments) GetSignature(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.AttachmentSignature, error) {
	return &domain.AttachmentSignature{EffectiveStatus: domain.VerificationPending, ReasonCode: domain.ReasonVerifierUnavailable, VerificationStatus: domain.VerificationUnverified, CreatedAt: time.Now()}, nil
}

func TestAttachSignatureRoutesBinaryAndRejectsClientStatus(t *testing.T) {
	fake := &fakeAttachments{}
	handler := &AttachmentHandler{service: fake}
	router := chi.NewRouter()
	router.Post("/v1/documents/{id}/attachments/{attachmentId}/signatures", handler.AttachSignature)
	tenant := uuid.New()
	path := "/v1/documents/" + uuid.NewString() + "/attachments/" + uuid.NewString() + "/signatures"
	jsonReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{"signature_format":"CAdES","signature_reference":"objects/sig-1","verification_status":"VALID"}`)))
	jsonReq.Header.Set("Content-Type", "application/json")
	jsonReq.Header.Set("X-Tenant-ID", tenant.String())
	jsonRec := httptest.NewRecorder()
	router.ServeHTTP(jsonRec, jsonReq)
	if jsonRec.Code != http.StatusBadRequest || fake.json != 0 {
		t.Fatalf("client status status=%d calls=%d", jsonRec.Code, fake.json)
	}
	binReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("sig")))
	binReq.Header.Set("Content-Type", "application/pkcs7-signature")
	binReq.Header.Set("Idempotency-Key", "bin-1")
	binReq.Header.Set("X-Tenant-ID", tenant.String())
	binRec := httptest.NewRecorder()
	router.ServeHTTP(binRec, binReq)
	if binRec.Code != http.StatusCreated || fake.detached != 1 || bytes.Contains(binRec.Body.Bytes(), []byte("tenants/")) {
		t.Fatalf("binary status=%d body=%s", binRec.Code, binRec.Body.String())
	}
	xmlReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("<sig/>")))
	xmlReq.Header.Set("Content-Type", "application/pkcs7-signature")
	xmlReq.Header.Set("X-Signature-Format", "XMLDSIG")
	xmlReq.Header.Set("Idempotency-Key", "bin-xml")
	xmlReq.Header.Set("X-Tenant-ID", tenant.String())
	xmlRec := httptest.NewRecorder()
	router.ServeHTTP(xmlRec, xmlReq)
	if xmlRec.Code != http.StatusBadRequest {
		t.Fatalf("xmldsig status=%d", xmlRec.Code)
	}
	for _, key := range []string{"", "   ", strings.Repeat("k", 129)} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("sig")))
		req.Header.Set("Content-Type", "application/pkcs7-signature")
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("X-Tenant-ID", tenant.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("key %q status=%d", key, rec.Code)
		}
	}
	if fake.detached != 1 {
		t.Fatalf("detached calls=%d", fake.detached)
	}
}
