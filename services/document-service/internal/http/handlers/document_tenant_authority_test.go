package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/service"
)

type listCaptureStore struct {
	*readStore
	seen *uuid.UUID
}

func (s *listCaptureStore) List(_ context.Context, filter domain.ListDocumentsFilter) ([]domain.Document, int, error) {
	*s.seen = filter.TenantID
	return nil, 0, nil
}

func TestDocumentListUsesTrustedTenant(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	var seen uuid.UUID
	handler := NewDocumentHandler(service.NewDocumentService(&listCaptureStore{readStore: &readStore{}, seen: &seen}))
	router := chi.NewRouter()
	router.Get("/v1/documents", handler.List)

	match := httptest.NewRequest(http.MethodGet, "/v1/documents?tenant_id="+tenantA.String(), nil)
	match.Header.Set("X-Tenant-ID", tenantA.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, match)
	if rec.Code != http.StatusOK || seen != tenantA {
		t.Fatalf("matching query status=%d tenant=%s", rec.Code, seen)
	}

	plain := httptest.NewRequest(http.MethodGet, "/v1/documents", nil)
	plain.Header.Set("X-Tenant-ID", tenantA.String())
	seen = uuid.Nil
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, plain)
	if rec.Code != http.StatusOK || seen != tenantA {
		t.Fatalf("missing query status=%d tenant=%s", rec.Code, seen)
	}

	conflict := httptest.NewRequest(http.MethodGet, "/v1/documents?tenant_id="+tenantB.String(), nil)
	conflict.Header.Set("X-Tenant-ID", tenantA.String())
	seen = uuid.Nil
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, conflict)
	if rec.Code != http.StatusUnauthorized || seen != uuid.Nil || strings.Contains(rec.Body.String(), tenantB.String()) {
		t.Fatalf("conflict status=%d seen=%s body=%s", rec.Code, seen, rec.Body.String())
	}
}

func TestDocumentWritesRejectBodyTenantOverride(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	docID := uuid.New()
	store := &readStore{}
	handler := NewDocumentHandler(service.NewDocumentService(store))
	router := chi.NewRouter()
	router.Post("/v1/documents", handler.Create)
	router.Post("/v1/documents/{id}/versions", handler.CreateVersion)
	router.Post("/v1/documents/{id}/files", handler.AddFile)
	router.Post("/v1/documents/{id}/ready-for-signing", handler.ReadyForSigning)
	router.Post("/v1/documents/{id}/cancel", handler.Cancel)
	router.Post("/v1/documents/{id}/archive", handler.Archive)

	owner := uuid.New()
	version := uuid.New()
	cases := []struct {
		path string
		body string
	}{
		{"/v1/documents", `{"tenant_id":"` + tenantB.String() + `","document_number":"N","document_type":"ETRN","owner_company_id":"` + owner.String() + `","legal_language":"ru-RU"}`},
		{"/v1/documents/" + docID.String() + "/versions", `{"tenant_id":"` + tenantB.String() + `","payload_json":{}}`},
		{"/v1/documents/" + docID.String() + "/files", `{"tenant_id":"` + tenantB.String() + `","document_version_id":"` + version.String() + `","file_type":"PDF","storage_provider":"S3","object_key":"k"}`},
		{"/v1/documents/" + docID.String() + "/ready-for-signing", `{"tenant_id":"` + tenantB.String() + `"}`},
		{"/v1/documents/" + docID.String() + "/cancel", `{"tenant_id":"` + tenantB.String() + `","reason":"stop"}`},
		{"/v1/documents/" + docID.String() + "/archive", `{"tenant_id":"` + tenantB.String() + `"}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-Tenant-ID", tenantA.String())
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || store.writes != 0 {
			t.Fatalf("%s status=%d writes=%d body=%s", tc.path, rec.Code, store.writes, rec.Body.String())
		}
	}
}
