//go:build integration

package erp

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/shared-go/integrationauth"
)

func enabledERPIntegrationConfig() config.Config {
	return config.Config{RfxErpIntegrationEnabled: true}
}

func newERPPreviewRouter(t *testing.T, env *testEnv, cfg config.Config) http.Handler {
	t.Helper()
	rfxRepo := repository.NewRfxRepository(env.pool)
	qRepo := repository.NewQuestionnaireRepository(env.pool)
	txRunner := repository.NewTransactionRunner(env.pool)
	mappingResolver := service.NewErpMappingResolver(env.mappingRepo)
	erpSvc := service.NewErpIntegrationService(rfxRepo, qRepo, env.analysisRepo, mappingResolver, nil, txRunner)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpserver.NewRouter(log, env.pool, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, erpSvc, nil, nil, nil, nil, nil)
}

func injectIntegrationHeaders(req *http.Request, tenantID, companyID, principalID uuid.UUID, scopes ...string) {
	integrationauth.InjectTrustedIntegrationHeaders(req.Header, integrationauth.AuthenticatedContext{
		TenantID:    tenantID,
		CompanyID:   companyID,
		PrincipalID: principalID,
		Scopes:      scopes,
		AuthScheme:  integrationauth.AuthSchemeOAuth,
	})
}

func postERPPreview(t *testing.T, router http.Handler, path string, tenantID, companyID, principalID uuid.UUID, scopes []string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	injectIntegrationHeaders(req, tenantID, companyID, principalID, scopes...)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func seedPlatformMappingSet(t *testing.T, env *testEnv, tenantID uuid.UUID, mappingType string, version int, externalCode, canonicalCode string) uuid.UUID {
	t.Helper()
	return seedMappingSet(t, env, tenantID, nil, mappingType, version, domain.ReferenceMappingSetStatusActive, externalCode, canonicalCode)
}

func seedTenantMappingSet(t *testing.T, env *testEnv, tenantID uuid.UUID, mappingType string, version int, externalCode, canonicalCode string) uuid.UUID {
	t.Helper()
	return seedMappingSet(t, env, tenantID, &tenantID, mappingType, version, domain.ReferenceMappingSetStatusActive, externalCode, canonicalCode)
}

func seedRetiredMappingSet(t *testing.T, env *testEnv, tenantID uuid.UUID, scopeTenant *uuid.UUID, mappingType string, version int, externalCode, canonicalCode string) uuid.UUID {
	t.Helper()
	return seedMappingSet(t, env, tenantID, scopeTenant, mappingType, version, domain.ReferenceMappingSetStatusRetired, externalCode, canonicalCode)
}

func seedMappingSet(
	t *testing.T,
	env *testEnv,
	entryTenantID uuid.UUID,
	scopeTenant *uuid.UUID,
	mappingType string,
	version int,
	status string,
	externalCode, canonicalCode string,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	set, err := env.mappingRepo.CreateSet(ctx, domain.ReferenceMappingSet{
		TenantID:    scopeTenant,
		MappingType: mappingType,
		Version:     version,
		Status:      status,
	})
	if err != nil {
		t.Fatalf("create mapping set: %v", err)
	}
	if externalCode != "" {
		if _, err := env.mappingRepo.CreateEntry(ctx, domain.ReferenceMappingEntry{
			TenantID:      entryTenantID,
			MappingSetID:  set.ID,
			ExternalCode:  externalCode,
			CanonicalCode: canonicalCode,
		}); err != nil {
			t.Fatalf("create mapping entry: %v", err)
		}
	}
	return set.ID
}

func seedPublishedRfxEvent(t *testing.T, env *testEnv, tenantID, companyID uuid.UUID) uuid.UUID {
	t.Helper()
	eventID := uuid.New()
	_, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_events (
			id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status
		) VALUES ($1, $2, $3, 'SPOT_RFQ', 'FREIGHT', 'ERP Published Event', $4, 'PUBLISHED')
	`, eventID, tenantID, "ERP-PUB-"+eventID.String()[:8], companyID)
	if err != nil {
		t.Fatalf("seed published event: %v", err)
	}
	return eventID
}

func seedDraftRfxEvent(t *testing.T, env *testEnv, tenantID, companyID uuid.UUID) uuid.UUID {
	t.Helper()
	eventID := seedRfxEvent(t, env, tenantID, companyID)
	qRepo := repository.NewQuestionnaireRepository(env.pool)
	if _, err := qRepo.GetOrCreateDraftVersion(context.Background(), tenantID, eventID); err != nil {
		t.Fatalf("draft version: %v", err)
	}
	return eventID
}

func postERPUpdatePreview(t *testing.T, router http.Handler, eventID, tenantID, companyID, principalID uuid.UUID, scopes []string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	path := "/v1/rfx-events/" + eventID.String() + "/erp-import/preview"
	return postERPPreview(t, router, path, tenantID, companyID, principalID, scopes, body)
}

func fetchAnalysisCanonicalJSON(t *testing.T, env *testEnv, analysisID uuid.UUID) []byte {
	t.Helper()
	payload, _ := fetchAnalysisCanonicalRecord(t, env, analysisID)
	return payload
}

func fetchAnalysisCanonicalRecord(t *testing.T, env *testEnv, analysisID uuid.UUID) (payload []byte, hash string) {
	t.Helper()
	if err := env.pool.QueryRow(context.Background(), `
		SELECT canonical_payload_json, canonical_hash FROM rfx.rfx_import_analyses WHERE id = $1
	`, analysisID).Scan(&payload, &hash); err != nil {
		t.Fatalf("fetch analysis canonical record: %v", err)
	}
	return payload, hash
}

func countERPAnalyses(t *testing.T, env *testEnv, tenantID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_import_analyses WHERE tenant_id = $1 AND workbook_type = 'ERP_BUYER_JSON'
	`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count analyses: %v", err)
	}
	return count
}
