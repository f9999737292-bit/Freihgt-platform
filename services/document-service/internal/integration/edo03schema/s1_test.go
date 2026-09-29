//go:build integration

package edo03schema

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/http/handlers"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

func TestDocumentReadTenantIsolationWithI1Schema(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	allow(t, applyI1(ctx, pool, false))
	docRepo := repository.NewDocumentRepository(pool)
	docs := service.NewDocumentService(docRepo)
	signing := service.NewSigningService(repository.NewSigningRepository(pool), docRepo)
	tenantA, tenantB := uuid.New(), uuid.New()
	companyA, companyB := uuid.New(), uuid.New()
	for i, row := range []struct{ company, tenant uuid.UUID }{{companyA, tenantA}, {companyB, tenantB}} {
		_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
			row.tenant, "t"+string(rune('A'+i)), "Tenant")
		allow(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, legal_name, company_type)
			VALUES ($1,$2,'Company','CARRIER')`, row.company, row.tenant)
		allow(t, err)
	}
	docA := mustCreate(t, docs, tenantA, companyA, "S1-OWN-A")
	docB := mustCreate(t, docs, tenantB, companyB, "S1-FOREIGN-B")
	allow(t, ignore(docs.ReadyForSigning(ctx, docA.ID, domain.ReadyForSigningInput{TenantID: tenantA})))
	sessionA, err := signing.CreateSession(ctx, docA.ID, domain.CreateSigningSessionInput{
		TenantID: tenantA, RequiredSignersCount: 1,
	})
	allow(t, err)

	router := chi.NewRouter()
	router.Get("/v1/documents/{id}", handlers.NewDocumentHandler(docs).GetByID)
	router.Get("/v1/signing-sessions/{id}", handlers.NewSigningHandler(signing).GetSession)

	own := doGet(router, "/v1/documents/"+docA.ID.String(), tenantA.String())
	if own.Code != http.StatusOK || !strings.Contains(own.Body.String(), "S1-OWN-A") {
		t.Fatalf("own document status=%d body=%s", own.Code, own.Body.String())
	}
	foreign := doGet(router, "/v1/documents/"+docB.ID.String()+"?tenant_id="+tenantB.String(), tenantA.String())
	missing := doGet(router, "/v1/documents/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", tenantA.String())
	if foreign.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || foreign.Body.String() != missing.Body.String() {
		t.Fatalf("foreign and missing differ foreign=%d missing=%d", foreign.Code, missing.Code)
	}
	if strings.Contains(foreign.Body.String(), "S1-FOREIGN-B") || strings.Contains(foreign.Body.String(), docB.ID.String()) {
		t.Fatal("foreign document response leaks data")
	}
	_, err = pool.Exec(ctx, `UPDATE documents.documents SET deleted_at = now() WHERE id = $1 AND tenant_id = $2`, docB.ID, tenantB)
	allow(t, err)
	deleted := doGet(router, "/v1/documents/"+docB.ID.String(), tenantB.String())
	if deleted.Code != http.StatusNotFound || deleted.Body.String() != foreign.Body.String() {
		t.Fatal("deleted document is distinguishable from missing")
	}
	noTenant := doGet(router, "/v1/documents/"+docA.ID.String(), "")
	if noTenant.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant status=%d", noTenant.Code)
	}
	ownSession := doGet(router, "/v1/signing-sessions/"+sessionA.ID.String(), tenantA.String())
	foreignSession := doGet(router, "/v1/signing-sessions/"+sessionA.ID.String(), tenantB.String())
	missingSession := doGet(router, "/v1/signing-sessions/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", tenantB.String())
	if ownSession.Code != http.StatusOK {
		t.Fatalf("own session status=%d", ownSession.Code)
	}
	if foreignSession.Code != http.StatusNotFound || missingSession.Code != http.StatusNotFound || foreignSession.Body.String() != missingSession.Body.String() {
		t.Fatal("foreign signing session is distinguishable from missing")
	}
}

func mustCreate(t *testing.T, docs *service.DocumentService, tenant, company uuid.UUID, number string) *domain.Document {
	t.Helper()
	doc, err := docs.Create(context.Background(), domain.CreateDocumentInput{
		TenantID: tenant, DocumentNumber: number, DocumentType: "ACT",
		OwnerCompanyID: company, LegalLanguage: "ru-RU",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return doc
}

func ignore(doc *domain.Document, err error) error {
	if doc == nil && err == nil {
		return nil
	}
	return err
}

func doGet(router http.Handler, path, tenant string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if tenant != "" {
		req.Header.Set("X-Tenant-ID", tenant)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
