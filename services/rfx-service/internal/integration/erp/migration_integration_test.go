//go:build integration

package erp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

func TestE7P2INT187Migration074UpDownUp(t *testing.T) {
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer dropDB(context.Background())
	pool, err := connectPool(ctx, testURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	up074 := filepath.Join(migrationsDir, "000074_rfx_erp_integration_principals_v3_0e7_phase2.up.sql")
	if _, err := os.Stat(up074); err != nil {
		t.Fatalf("missing migration 000074 up: %v", err)
	}
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("up all: %v", err)
	}
	downSQL, _ := os.ReadFile(filepath.Join(migrationsDir, "000074_rfx_erp_integration_principals_v3_0e7_phase2.down.sql"))
	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("down 000074: %v", err)
	}
	upSQL, _ := os.ReadFile(up074)
	if _, err := pool.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("up 000074 again: %v", err)
	}
}

func TestE7P2INT187Migration074DownSearchPath(t *testing.T) {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	down, err := os.ReadFile(filepath.Join(migrationsDir, "000074_rfx_erp_integration_principals_v3_0e7_phase2.down.sql"))
	if err != nil {
		t.Fatalf("read down: %v", err)
	}
	if !strings.Contains(string(down), "SET search_path = public") {
		t.Fatal("down migration must set search_path=public")
	}
}

func TestIntegrationPrincipalUniquenessAndTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantA, companyA := seedTenantCompany(t, env)
	tenantB, companyB := seedTenantCompany(t, env)
	if _, err := env.principalRepo.Create(ctx, domain.IntegrationPrincipal{
		TenantID: tenantA, CompanyID: companyA, ClientID: "Acme-ERP",
		CredentialType: domain.IntegrationCredentialTypeOAuth,
		Status:         domain.IntegrationPrincipalStatusActive,
	}); err != nil {
		t.Fatalf("create principal A: %v", err)
	}
	if _, err := env.principalRepo.Create(ctx, domain.IntegrationPrincipal{
		TenantID: tenantB, CompanyID: companyB, ClientID: "acme-erp",
		CredentialType: domain.IntegrationCredentialTypeOAuth,
		Status:         domain.IntegrationPrincipalStatusActive,
	}); err != nil {
		t.Fatalf("same client_id allowed in other tenant: %v", err)
	}
	if _, err := env.principalRepo.Create(ctx, domain.IntegrationPrincipal{
		TenantID: tenantA, CompanyID: companyA, ClientID: "ACME-ERP",
		CredentialType: domain.IntegrationCredentialTypeOAuth,
		Status:         domain.IntegrationPrincipalStatusActive,
	}); err == nil {
		t.Fatal("expected duplicate client_id rejection within tenant")
	}
}

func TestCredentialHashOnlyStorage(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "cred-client")
	cred, err := env.credentialRepo.StoreHash(ctx, domain.IntegrationCredential{
		TenantID:               tenantID,
		IntegrationPrincipalID: principal.ID,
		CredentialType:         domain.IntegrationCredentialTypeAPIKey,
		SecretHash:             "bcrypt$2a$12$hashonly",
		HashAlgorithm:          "bcrypt",
		HashVersion:            1,
		LookupFingerprint:      "bt_live_prefix",
	})
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}
	if cred.SecretHash == "" {
		t.Fatal("expected secret_hash persisted")
	}
	absent, err := env.credentialRepo.CredentialPlaintextAbsent(ctx)
	if err != nil || !absent {
		t.Fatalf("plaintext secret columns must be absent: absent=%v err=%v", absent, err)
	}
}

func TestScopeUniquenessFailClosed(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "scope-client")
	if err := env.scopeRepo.Grant(ctx, tenantID, principal.ID, domain.ScopeDraftCreate); err != nil {
		t.Fatalf("grant scope: %v", err)
	}
	if err := env.scopeRepo.Grant(ctx, tenantID, principal.ID, domain.ScopeDraftCreate); err != nil {
		t.Fatalf("idempotent grant: %v", err)
	}
	if err := env.scopeRepo.Grant(ctx, tenantID, principal.ID, "rfx:invalid:scope"); err == nil {
		t.Fatal("expected invalid scope rejection")
	}
	has, err := env.scopeRepo.HasScope(ctx, tenantID, principal.ID, domain.ScopeDraftCreate)
	if err != nil || !has {
		t.Fatalf("scope lookup failed: has=%v err=%v", has, err)
	}
}

func TestAnalysisXORAndLegacyXLSXInsert(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	actorID := uuid.New()
	rawPayload := []byte(`{"schema_name":"BINTRANS_RFX_BUYER_XLSX_V1","schema_version":"BINTRANS_RFX_BUYER_XLSX_V1","mode":"NEW_EVENT","lots":[],"questionnaire":{"sections":[],"questions":[],"options":[],"rules":[]},"counts":{"errors":0,"warnings":0,"lots":0,"sections":0,"questions":0,"options":0,"rules":0}}`)
	payload, hash, err := xlsxexchange.StableStoredPayload(rawPayload)
	if err != nil {
		t.Fatalf("stable payload: %v", err)
	}
	analysis, err := env.analysisRepo.CreatePreview(ctx, domain.ImportAnalysis{
		TenantID: tenantID, ActorID: actorID, ActorCompanyID: companyID,
		WorkbookType: domain.WorkbookTypeBuyerTender, SchemaVersion: domain.SchemaVersionBuyerXLSXV1,
		TargetType: domain.ImportTargetTypeNewEvent, CanonicalPayloadJSON: payload, CanonicalHash: hash,
		ValidationSummary: []byte(`{}`), ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("legacy xlsx insert: %v", err)
	}
	if analysis.ActorID != actorID || analysis.IntegrationPrincipalID != nil {
		t.Fatalf("unexpected owner fields: %+v", analysis)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_import_analyses (
			tenant_id, actor_id, integration_principal_id, actor_company_id, workbook_type, schema_version,
			target_type, canonical_payload_json, canonical_hash, status, validation_summary, expires_at
		) VALUES ($1, NULL, NULL, $2, 'BUYER_TENDER', 'BINTRANS_RFX_BUYER_XLSX_V1', 'NEW_EVENT',
			'{}'::jsonb, $3, 'PREVIEWED', '{}'::jsonb, now() + interval '1 hour')
	`, tenantID, companyID, hash)
	if err == nil {
		t.Fatal("expected both-null owner rejection")
	}
}

func TestERPPrincipalOwnedAnalysisInsertFoundation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "analysis-client")
	payload := []byte(`{"event":{"title":"ERP"}}`)
	hash := canonicalHashPayload(payload)
	analysis, err := env.analysisRepo.CreatePreviewForIntegrationPrincipal(ctx, domain.ImportAnalysis{
		TenantID: tenantID, IntegrationPrincipalID: &principal.ID, ActorCompanyID: companyID,
		WorkbookType: domain.WorkbookTypeERPBuyerJSON, SchemaVersion: domain.SchemaVersionERPJSONV1,
		TargetType: domain.ImportTargetTypeNewEvent, CanonicalPayloadJSON: payload, CanonicalHash: hash,
		ValidationSummary: []byte(`{}`), ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("erp analysis insert: %v", err)
	}
	if analysis.IntegrationPrincipalID == nil || *analysis.IntegrationPrincipalID != principal.ID {
		t.Fatalf("unexpected principal owner: %+v", analysis)
	}
	if analysis.ActorID != uuid.Nil {
		t.Fatal("actor_id must be empty for ERP analysis")
	}
}

func TestHumanIdempotencyReplayPreserved(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, _ := seedTenantCompany(t, env)
	actorID := uuid.New()
	scope := repository.IdempotencyScope{
		TenantID: tenantID, ActorID: actorID, Operation: "TEST_HUMAN", AggregateScope: uuid.New(),
	}
	record := repository.IdempotencyRecord{
		TenantID: tenantID, ActorID: actorID, Operation: scope.Operation, AggregateScope: scope.AggregateScope,
		IdempotencyKey: "key-1", RequestBodyHash: "abc", ResponseStatus: 200,
		ResponseBody: json.RawMessage(`{"ok":true}`), ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := env.idemRepo.Store(ctx, record); err != nil {
		t.Fatalf("store human idempotency: %v", err)
	}
	got, err := env.idemRepo.Get(ctx, scope, "key-1")
	if err != nil || got == nil {
		t.Fatalf("human replay lookup failed: got=%v err=%v", got, err)
	}
}

func TestPrincipalIdempotencyIsolationAndCrossPrincipalReplayDenied(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	principalA := seedIntegrationPrincipal(t, env, tenantID, companyID, "idem-a")
	principalB := seedIntegrationPrincipal(t, env, tenantID, companyID, "idem-b")
	aggregate := uuid.New()
	key := "shared-key"
	recordA := repository.IdempotencyRecord{
		TenantID: tenantID, IntegrationPrincipalID: principalA.ID, OwnerKind: domain.OwnerKindIntegrationPrincipal,
		Operation: "ERP_BUYER_CREATE_COMMIT", AggregateScope: aggregate, IdempotencyKey: key,
		RequestBodyHash: "hash-a", ResponseStatus: 201, ResponseBody: json.RawMessage(`{}`),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := env.idemRepo.Store(ctx, recordA); err != nil {
		t.Fatalf("store principal idempotency: %v", err)
	}
	scopeB := repository.IdempotencyScope{
		TenantID: tenantID, IntegrationPrincipalID: principalB.ID, OwnerKind: domain.OwnerKindIntegrationPrincipal,
		Operation: "ERP_BUYER_CREATE_COMMIT", AggregateScope: aggregate,
	}
	got, err := env.idemRepo.Get(ctx, scopeB, key)
	if err != nil || got != nil {
		t.Fatalf("cross-principal replay must miss: got=%v err=%v", got, err)
	}
	humanScope := repository.IdempotencyScope{
		TenantID: tenantID, ActorID: uuid.New(), Operation: "ERP_BUYER_CREATE_COMMIT", AggregateScope: aggregate,
	}
	gotHuman, err := env.idemRepo.Get(ctx, humanScope, key)
	if err != nil || gotHuman != nil {
		t.Fatalf("human/principal namespaces must be isolated: got=%v err=%v", gotHuman, err)
	}
}

func TestStableExternalIdentityWithoutRevisionInUniqueKey(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "external-client")
	eventID := seedRfxEvent(t, env, tenantID, companyID)
	hash := strings.Repeat("a", 64)
	link, err := env.externalLinkRepo.UpsertLink(ctx, domain.ExternalObjectLink{
		TenantID: tenantID, IntegrationPrincipalID: principal.ID, ExternalSystem: "SAP",
		ExternalObjectType: "RFX_EVENT", ExternalObjectID: "OBJ-1", ExternalVersion: "1",
		ExternalRevision: "1", PayloadHash: hash, RfxEventID: eventID,
	})
	if err != nil {
		t.Fatalf("upsert link: %v", err)
	}
	updated, err := env.externalLinkRepo.UpsertLink(ctx, domain.ExternalObjectLink{
		TenantID: tenantID, IntegrationPrincipalID: principal.ID, ExternalSystem: "SAP",
		ExternalObjectType: "RFX_EVENT", ExternalObjectID: "OBJ-1", ExternalVersion: "2",
		ExternalRevision: "2", PayloadHash: strings.Repeat("b", 64), RfxEventID: eventID,
	})
	if err != nil {
		t.Fatalf("update same stable identity: %v", err)
	}
	if updated.ID != link.ID {
		t.Fatalf("stable identity must update same row: old=%s new=%s", link.ID, updated.ID)
	}
	if updated.ExternalRevision != "2" {
		t.Fatalf("revision metadata expected 2, got %q", updated.ExternalRevision)
	}
	_, err = env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_external_object_links (
			tenant_id, integration_principal_id, external_system, external_object_type,
			external_object_id, external_version, external_revision, payload_hash, rfx_event_id
		) VALUES ($1, $2, 'SAP', 'RFX_EVENT', 'OBJ-1', '3', '3', $3, $4)
	`, tenantID, principal.ID, hash, eventID)
	if err == nil {
		t.Fatal("duplicate stable identity must fail closed")
	}
}

func TestExternalRevisionHistoryMetadata(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "revision-client")
	eventID := seedRfxEvent(t, env, tenantID, companyID)
	link, err := env.externalLinkRepo.UpsertLink(ctx, domain.ExternalObjectLink{
		TenantID: tenantID, IntegrationPrincipalID: principal.ID, ExternalSystem: "SAP",
		ExternalObjectType: "RFX_EVENT", ExternalObjectID: "OBJ-REV", ExternalVersion: "1",
		ExternalRevision: "1", PayloadHash: strings.Repeat("c", 64), RfxEventID: eventID,
	})
	if err != nil {
		t.Fatalf("upsert link: %v", err)
	}
	rev, err := env.externalLinkRepo.RecordRevision(ctx, domain.ExternalObjectLinkRevision{
		TenantID: tenantID, LinkID: link.ID, ExternalRevision: "1", PayloadHash: strings.Repeat("c", 64),
	})
	if err != nil || rev.ID == uuid.Nil {
		t.Fatalf("record revision: rev=%v err=%v", rev, err)
	}
}

func TestReferenceMappingConstraints(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, _ := seedTenantCompany(t, env)
	set, err := env.mappingRepo.CreateSet(ctx, domain.ReferenceMappingSet{
		TenantID: &tenantID, MappingType: "CURRENCY", Version: 1, Status: domain.ReferenceMappingSetStatusActive,
	})
	if err != nil {
		t.Fatalf("create mapping set: %v", err)
	}
	if _, err := env.mappingRepo.CreateEntry(ctx, domain.ReferenceMappingEntry{
		TenantID: tenantID, MappingSetID: set.ID, ExternalCode: "USD", CanonicalCode: "USD",
	}); err != nil {
		t.Fatalf("create mapping entry: %v", err)
	}
	count, err := env.mappingRepo.CountEntriesForExternalCode(ctx, set.ID, "USD")
	if err != nil || count != 1 {
		t.Fatalf("ambiguous mapping count: count=%d err=%v", count, err)
	}
	if _, err := env.mappingRepo.CreateSet(ctx, domain.ReferenceMappingSet{
		TenantID: &tenantID, MappingType: "CURRENCY", Version: 0, Status: domain.ReferenceMappingSetStatusActive,
	}); err == nil {
		t.Fatal("expected invalid version rejection")
	}
}

func TestDownMigrationGuards(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "down-guard")
	payload := []byte(`{"event":{"title":"ERP"}}`)
	hash := canonicalHashPayload(payload)
	if _, err := env.analysisRepo.CreatePreviewForIntegrationPrincipal(ctx, domain.ImportAnalysis{
		TenantID: tenantID, IntegrationPrincipalID: &principal.ID, ActorCompanyID: companyID,
		WorkbookType: domain.WorkbookTypeERPBuyerJSON, SchemaVersion: domain.SchemaVersionERPJSONV1,
		TargetType: domain.ImportTargetTypeNewEvent, CanonicalPayloadJSON: payload, CanonicalHash: hash,
		ValidationSummary: []byte(`{}`), ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatalf("seed erp analysis: %v", err)
	}
	if err := applyMigrationFile(ctx, env.pool, "000074_rfx_erp_integration_principals_v3_0e7_phase2.down.sql"); err == nil {
		t.Fatal("down migration must be blocked when ERP-owned analyses exist")
	}
}

func TestLegacy073FixtureMigratesWithoutDataLoss(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	tenantID, companyID := seedTenantCompany(t, env)
	analysisID := uuid.New()
	payload := []byte(`{"event":{"title":"Legacy"}}`)
	hash := canonicalHashPayload(payload)
	_, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_import_analyses (
			id, tenant_id, actor_id, actor_company_id, workbook_type, schema_version,
			target_type, canonical_payload_json, canonical_hash, status, validation_summary, expires_at
		) VALUES (
			$1, $2, $3, $4, 'BUYER_TENDER', 'BINTRANS_RFX_BUYER_XLSX_V1', 'NEW_EVENT',
			$5, $6, 'PREVIEWED', '{}'::jsonb, now() + interval '1 hour'
		)
	`, analysisID, tenantID, uuid.New(), companyID, payload, hash)
	if err != nil {
		t.Fatalf("seed legacy analysis: %v", err)
	}
	var actor uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		SELECT actor_id FROM rfx.rfx_import_analyses WHERE id = $1
	`, analysisID).Scan(&actor); err != nil || actor == uuid.Nil {
		t.Fatalf("legacy actor_id lost after 000074: actor=%v err=%v", actor, err)
	}
}
