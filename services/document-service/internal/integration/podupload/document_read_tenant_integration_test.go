//go:build integration

package podupload

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

func TestDocumentReadTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedFixtures(t, env.pool)
	ctx := context.Background()
	docRepo := repository.NewDocumentRepository(env.pool)
	docs := service.NewDocumentService(docRepo)
	signing := service.NewSigningService(repository.NewSigningRepository(env.pool), docRepo)
	docA := mustCreateDocument(t, docs, fix.TenantA, fix.CompanyA, "S1-OWN-A")
	docB := mustCreateDocument(t, docs, fix.TenantB, fix.CompanyB, "S1-FOREIGN-B")
	if _, err := docs.ReadyForSigning(ctx, docA.ID, domain.ReadyForSigningInput{TenantID: fix.TenantA}); err != nil {
		t.Fatalf("ready for signing: %v", err)
	}
	sessionA, err := signing.CreateSession(ctx, docA.ID, domain.CreateSigningSessionInput{
		TenantID: fix.TenantA, RequiredSignersCount: 1,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	router := chi.NewRouter()
	docHandler := handlers.NewDocumentHandler(docs)
	sessionHandler := handlers.NewSigningHandler(signing)
	router.Get("/v1/documents", docHandler.List)
	router.Get("/v1/documents/{id}", docHandler.GetByID)
	router.Get("/v1/signing-sessions/{id}", sessionHandler.GetSession)

	before := snapshotCounts(t, env)
	own := doGet(router, "/v1/documents/"+docA.ID.String(), fix.TenantA.String())
	if own.Code != http.StatusOK || !strings.Contains(own.Body.String(), "S1-OWN-A") || strings.Contains(own.Body.String(), "S1-FOREIGN-B") {
		t.Fatalf("own document status=%d", own.Code)
	}
	foreign := doGet(router, "/v1/documents/"+docB.ID.String()+"?tenant_id="+fix.TenantB.String(), fix.TenantA.String())
	missing := doGet(router, "/v1/documents/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", fix.TenantA.String())
	if foreign.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || foreign.Body.String() != missing.Body.String() {
		t.Fatal("foreign and missing document responses differ")
	}
	if strings.Contains(foreign.Body.String(), "S1-FOREIGN-B") || strings.Contains(foreign.Body.String(), docB.ID.String()) || strings.Contains(foreign.Body.String(), fix.TenantB.String()) || strings.Contains(foreign.Body.String(), fix.CompanyB.String()) {
		t.Fatal("foreign document response leaks tenant data")
	}
	notFoundBody := foreign.Body.String()
	if _, err := env.pool.Exec(ctx, `UPDATE documents.documents SET deleted_at = now() WHERE id = $1 AND tenant_id = $2`, docB.ID, fix.TenantB); err != nil {
		t.Fatal(err)
	}
	deleted := doGet(router, "/v1/documents/"+docB.ID.String(), fix.TenantB.String())
	if deleted.Code != http.StatusNotFound || deleted.Body.String() != notFoundBody {
		t.Fatal("deleted document is distinguishable from missing")
	}

	noTenant := doGet(router, "/v1/documents/"+docA.ID.String(), "")
	badTenant := doGet(router, "/v1/documents/"+docA.ID.String(), "not-a-uuid")
	if noTenant.Code != http.StatusUnauthorized || badTenant.Code != http.StatusUnauthorized {
		t.Fatal("downstream read without trusted tenant did not fail closed")
	}
	if strings.Contains(noTenant.Body.String(), "S1-OWN-A") || strings.Contains(noTenant.Body.String(), docA.ID.String()) {
		t.Fatal("untrusted read returned document data")
	}

	if doGet(router, "/v1/documents/"+docA.ID.String(), fix.TenantA.String()).Code != http.StatusOK {
		t.Fatal("repeat get failed")
	}
	if before != snapshotCounts(t, env) {
		t.Fatal("get changed stored row counts")
	}

	list := doGet(router, "/v1/documents?tenant_id="+fix.TenantA.String(), "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), docA.ID.String()) || strings.Contains(list.Body.String(), docB.ID.String()) {
		t.Fatal("list tenant filter regression")
	}
	if _, err := docs.CreateVersion(ctx, docA.ID, domain.CreateDocumentVersionInput{TenantID: fix.TenantB}); err == nil {
		t.Fatal("cross-tenant version create succeeded")
	}

	ownSession := doGet(router, "/v1/signing-sessions/"+sessionA.ID.String(), fix.TenantA.String())
	foreignSession := doGet(router, "/v1/signing-sessions/"+sessionA.ID.String()+"?tenant_id="+fix.TenantA.String(), fix.TenantB.String())
	missingSession := doGet(router, "/v1/signing-sessions/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", fix.TenantB.String())
	if ownSession.Code != http.StatusOK {
		t.Fatal("own signing session was not readable")
	}
	if foreignSession.Code != http.StatusNotFound || missingSession.Code != http.StatusNotFound || foreignSession.Body.String() != missingSession.Body.String() {
		t.Fatal("foreign signing session is distinguishable from missing")
	}
	if strings.Contains(foreignSession.Body.String(), sessionA.ID.String()) || strings.Contains(foreignSession.Body.String(), docA.ID.String()) || strings.Contains(foreignSession.Body.String(), fix.TenantA.String()) {
		t.Fatal("foreign signing session response leaks identifiers")
	}
}

func mustCreateDocument(t *testing.T, docs *service.DocumentService, tenant, company uuid.UUID, number string) *domain.Document {
	t.Helper()
	doc, err := docs.Create(context.Background(), domain.CreateDocumentInput{
		TenantID: tenant, DocumentNumber: number, DocumentType: "ACT",
		OwnerCompanyID: company, LegalLanguage: "ru-RU",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	return doc
}

type rowCounts struct {
	documents, versions, files, sessions, signatures, intents int
}

func snapshotCounts(t *testing.T, env *testEnv) rowCounts {
	t.Helper()
	var counts rowCounts
	scan := func(query string, dest *int) {
		t.Helper()
		if err := env.pool.QueryRow(context.Background(), query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	scan(`SELECT COUNT(*) FROM documents.documents`, &counts.documents)
	scan(`SELECT COUNT(*) FROM documents.document_versions`, &counts.versions)
	scan(`SELECT COUNT(*) FROM documents.document_files`, &counts.files)
	scan(`SELECT COUNT(*) FROM documents.signing_sessions`, &counts.sessions)
	scan(`SELECT COUNT(*) FROM documents.signatures`, &counts.signatures)
	scan(`SELECT COUNT(*) FROM documents.document_upload_intent`, &counts.intents)
	return counts
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
