//go:build integration

package erp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

func createDraftPayload(objectID string) string {
	return `{
		"schema_version":"BINTRANS_RFX_ERP_JSON_V1",
		"requested_operation":"CREATE_DRAFT",
		"external":{"system":"SAP","object_id":"` + objectID + `"},
		"event":{"type":"SPOT_RFQ","title":"ERP Create Commit","currency":"USD_EXT","timezone":"UTC_EXT"}
	}`
}

func seedCreateCommitReady(t *testing.T, env *testEnv, objectID string) (tenantID, companyID uuid.UUID, principal *domain.IntegrationPrincipal, router http.Handler) {
	t.Helper()
	tenantID, companyID, principal, router, _ = seedCreateCommitReadyWithService(t, env, objectID)
	return tenantID, companyID, principal, router
}

func seedCreateCommitReadyWithService(t *testing.T, env *testEnv, objectID string) (tenantID, companyID uuid.UUID, principal *domain.IntegrationPrincipal, router http.Handler, erpSvc *service.ErpIntegrationService) {
	t.Helper()
	tenantID, companyID = seedTenantCompany(t, env)
	seedPlatformMappingSet(t, env, tenantID, "CURRENCY", 1, "USD_EXT", "USD")
	seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 1, "UTC_EXT", "UTC")
	principal = seedIntegrationPrincipal(t, env, tenantID, companyID, "e4-"+objectID)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftCreate)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftCommit)
	router, erpSvc = newERPPreviewRouterAndService(t, env, enabledERPIntegrationConfig())
	return tenantID, companyID, principal, router, erpSvc
}

func fetchAnalysisStatus(t *testing.T, env *testEnv, analysisID uuid.UUID) string {
	t.Helper()
	var status string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT status FROM rfx.rfx_import_analyses WHERE id = $1
	`, analysisID).Scan(&status); err != nil {
		t.Fatalf("fetch analysis status: %v", err)
	}
	return status
}

func countIdempotencyRecords(t *testing.T, env *testEnv, tenantID, principalID uuid.UUID, key string) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records
		WHERE tenant_id = $1 AND integration_principal_id = $2 AND idempotency_key = $3
	`, tenantID, principalID, key).Scan(&count); err != nil {
		t.Fatalf("count idempotency records: %v", err)
	}
	return count
}

func armCreateCommitIdempotencyMissBarrier(t *testing.T, erpSvc *service.ErpIntegrationService) *int32 {
	t.Helper()
	var arrived int32
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	go func() {
		<-ready
		<-ready
		close(release)
	}()
	erpSvc.SetAfterCreateCommitIdempotencyMiss(func() {
		atomic.AddInt32(&arrived, 1)
		ready <- struct{}{}
		select {
		case <-release:
		case <-time.After(10 * time.Second):
			t.Error("timed out waiting for concurrent CREATE commit peer after idempotency miss")
		}
	})
	t.Cleanup(func() { erpSvc.SetAfterCreateCommitIdempotencyMiss(nil) })
	return &arrived
}

func previewCreateAnalysis(t *testing.T, router http.Handler, tenantID, companyID, principalID uuid.UUID, objectID string) uuid.UUID {
	t.Helper()
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principalID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(createDraftPayload(objectID)))
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	raw, _ := out["analysis_id"].(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("analysis_id: %v", err)
	}
	return id
}

func decodeCreateCommit(t *testing.T, rec *http.Response, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode commit: %v body=%s", err, string(body))
	}
	return out
}

func errorMachineCode(body []byte) string {
	var env struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &env)
	code, _ := env.Error.Details["machine_code"].(string)
	return code
}

func countERPEvents(t *testing.T, env *testEnv, tenantID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id = $1 AND deleted_at IS NULL
	`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	return count
}

func countExternalLinks(t *testing.T, env *testEnv, tenantID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_external_object_links WHERE tenant_id = $1
	`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count links: %v", err)
	}
	return count
}

func fetchEventChannelStatus(t *testing.T, env *testEnv, eventID uuid.UUID) (channel, status string) {
	t.Helper()
	if err := env.pool.QueryRow(context.Background(), `
		SELECT creation_channel, status FROM rfx.rfx_events WHERE id = $1
	`, eventID).Scan(&channel, &status); err != nil {
		t.Fatalf("fetch event: %v", err)
	}
	return channel, status
}

func TestE7P2INT132MissingCommitScope(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT132")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT132")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, []string{domain.ScopeDraftCreate}, analysisID, "key-132")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT137CreateCommit201(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT137")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT137")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-137")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeCreateCommit(t, rec.Result(), rec.Body.Bytes())
	if out["creation_channel"] != domain.CreationChannelERP {
		t.Fatalf("creation_channel=%v", out["creation_channel"])
	}
	eventID, err := uuid.Parse(out["rfx_event_id"].(string))
	if err != nil {
		t.Fatalf("rfx_event_id: %v", err)
	}
	if _, err := uuid.Parse(out["external_link_id"].(string)); err != nil {
		t.Fatalf("external_link_id: %v", err)
	}
	if strings.TrimSpace(out["external_revision"].(string)) == "" {
		t.Fatal("expected external_revision")
	}
	channel, status := fetchEventChannelStatus(t, env, eventID)
	if channel != domain.CreationChannelERP || status != domain.RfxStatusDraft {
		t.Fatalf("channel=%s status=%s", channel, status)
	}
}

func TestE7P2INT138StableExternalLinkInserted(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT138")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT138")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-138")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeCreateCommit(t, rec.Result(), rec.Body.Bytes())
	link, err := env.externalLinkRepo.GetByStableIdentity(context.Background(), tenantID, principal.ID, "SAP", domain.ExternalObjectTypeRfxEvent, "INT138")
	if err != nil {
		t.Fatalf("stable link: %v", err)
	}
	if link.ID.String() != out["external_link_id"].(string) {
		t.Fatalf("link id mismatch %s vs %v", link.ID, out["external_link_id"])
	}
	if link.RfxEventID.String() != out["rfx_event_id"].(string) {
		t.Fatal("link event mismatch")
	}
}

func TestE7P2INT139DuplicateCreate409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT139")
	first := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT139")
	second := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT139")
	rec1 := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), first, "key-139-a")
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", rec1.Code, rec1.Body.String())
	}
	beforeEvents := countERPEvents(t, env, tenantID)
	rec2 := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), second, "key-139-b")
	if rec2.Code != http.StatusConflict || errorMachineCode(rec2.Body.Bytes()) != domain.MachineCodeExternalIDConflict {
		t.Fatalf("status=%d code=%s body=%s", rec2.Code, errorMachineCode(rec2.Body.Bytes()), rec2.Body.String())
	}
	if countERPEvents(t, env, tenantID) != beforeEvents {
		t.Fatal("duplicate CREATE must not leave an orphan event")
	}
}

func TestE7P2INT140CreateIdempotentReplay(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT140")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT140")
	rec1 := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-140")
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", rec1.Code, rec1.Body.String())
	}
	rec2 := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-140")
	if rec2.Code != http.StatusCreated {
		t.Fatalf("replay status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Fatalf("replay body mismatch %s vs %s", rec1.Body.String(), rec2.Body.String())
	}
	if countERPEvents(t, env, tenantID) != 1 || countExternalLinks(t, env, tenantID) != 1 {
		t.Fatal("replay must not create a second event or link")
	}
}

func TestE7P2INT141CommitWithoutPreview404(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT141")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), uuid.New(), "key-141")
	if rec.Code != http.StatusNotFound || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeAnalysisNotFound {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
	if countERPEvents(t, env, tenantID) != 0 {
		t.Fatal("missing analysis must write zero events")
	}
}

func TestE7P2INT144MissingIdempotencyKey400(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT144")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT144")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT145IdempotencyConflict409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT145")
	first := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT145-A")
	second := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT145-B")
	rec1 := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), first, "shared-145")
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", rec1.Code, rec1.Body.String())
	}
	rec2 := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), second, "shared-145")
	if rec2.Code != http.StatusConflict || errorMachineCode(rec2.Body.Bytes()) != domain.MachineCodeIdempotencyConflict {
		t.Fatalf("status=%d code=%s body=%s", rec2.Code, errorMachineCode(rec2.Body.Bytes()), rec2.Body.String())
	}
	if countERPEvents(t, env, tenantID) != 1 {
		t.Fatal("idempotency conflict must not create a second event")
	}
}

func TestE7P2INT177CommitDoesNotPublish(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT177")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT177")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-177")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeCreateCommit(t, rec.Result(), rec.Body.Bytes())
	eventID := uuid.MustParse(out["rfx_event_id"].(string))
	_, status := fetchEventChannelStatus(t, env, eventID)
	if status != domain.RfxStatusDraft {
		t.Fatalf("status=%s", status)
	}
	var published int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_versions WHERE rfx_event_id = $1 AND status = 'PUBLISHED'
	`, eventID).Scan(&published); err != nil {
		t.Fatalf("count published: %v", err)
	}
	if published != 0 {
		t.Fatal("commit must not publish")
	}
}

func TestE7P2INT180AuditCreateCommit(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT180")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT180")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-180")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeCreateCommit(t, rec.Result(), rec.Body.Bytes())
	eventID := uuid.MustParse(out["rfx_event_id"].(string))
	auditRepo := repository.NewAuditRepository(env.pool)
	events, err := auditRepo.ListByEntity(context.Background(), tenantID, "rfx_event", eventID, 20)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Action != domain.ERPCreateCommitAuditAction {
			continue
		}
		found = true
		if ev.ActorUserID != nil {
			t.Fatal("ERP audit must not synthesize a human actor_id")
		}
		raw, _ := json.Marshal(ev.Metadata)
		blob := strings.ToLower(string(raw))
		for _, secret := range []string{"password", "secret", "token", "api_key", "authorization"} {
			if strings.Contains(blob, secret) {
				t.Fatalf("audit metadata must not contain %s", secret)
			}
		}
		if ev.Metadata["actor_kind"] != domain.AuditActorKindIntegration {
			t.Fatalf("actor_kind=%v", ev.Metadata["actor_kind"])
		}
	}
	if !found {
		t.Fatal("expected rfx.erp.draft.created.v1")
	}
}

func TestE7P2INT191AnalysisPrincipalBinding(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principalA, router := seedCreateCommitReady(t, env, "INT191")
	principalB := seedIntegrationPrincipal(t, env, tenantID, companyID, "e4-int191-b")
	grantScope(t, env, tenantID, principalB.ID, domain.ScopeDraftCreate)
	grantScope(t, env, tenantID, principalB.ID, domain.ScopeDraftCommit)
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principalA.ID, "INT191")
	rec := postERPCreateCommit(t, router, tenantID, companyID, principalB.ID, commitScopes(), analysisID, "key-191")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeActorBindingDenied {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
	if countERPEvents(t, env, tenantID) != 0 {
		t.Fatal("binding denial must write zero events")
	}
}

func TestE7P2INT193StaleMappingContext409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT193")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT193")
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_reference_mapping_sets SET status = $1 WHERE tenant_id IS NULL
	`, domain.ReferenceMappingSetStatusRetired); err != nil {
		t.Fatalf("retire mapping sets: %v", err)
	}
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "key-193")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeStaleMappingContext {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
	if countERPEvents(t, env, tenantID) != 0 {
		t.Fatal("stale mapping must write zero events")
	}
}

func TestE7P2INT194ConcurrentCreateRace(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "INT194")
	first := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT194")
	second := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "INT194")
	var (
		wg   sync.WaitGroup
		rec1 *http.Response
		rec2 *http.Response
		b1   []byte
		b2   []byte
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		out := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), first, "key-194-a")
		rec1 = out.Result()
		b1 = out.Body.Bytes()
	}()
	go func() {
		defer wg.Done()
		out := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), second, "key-194-b")
		rec2 = out.Result()
		b2 = out.Body.Bytes()
	}()
	wg.Wait()
	codes := []int{rec1.StatusCode, rec2.StatusCode}
	created, conflict := 0, 0
	for i, code := range codes {
		body := b1
		if i == 1 {
			body = b2
		}
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			if errorMachineCode(body) != domain.MachineCodeExternalIDConflict {
				t.Fatalf("loser machine_code=%s body=%s", errorMachineCode(body), string(body))
			}
			conflict++
		default:
			t.Fatalf("unexpected status %d body=%s", code, string(body))
		}
	}
	if created != 1 || conflict != 1 {
		t.Fatalf("expected one 201 and one 409, got created=%d conflict=%d", created, conflict)
	}
	if countERPEvents(t, env, tenantID) != 1 || countExternalLinks(t, env, tenantID) != 1 {
		t.Fatal("concurrent CREATE must leave exactly one event and one link")
	}
}

func TestE4CreateCommitConcurrentIdempotencyConflictDifferentBodies(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router, erpSvc := seedCreateCommitReadyWithService(t, env, "RACE-DIFF")
	first := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "RACE-DIFF-A")
	second := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "RACE-DIFF-B")
	arrived := armCreateCommitIdempotencyMissBarrier(t, erpSvc)

	var (
		wg   sync.WaitGroup
		rec1 *http.Response
		rec2 *http.Response
		b1   []byte
		b2   []byte
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		out := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), first, "shared-race-diff")
		rec1 = out.Result()
		b1 = out.Body.Bytes()
	}()
	go func() {
		defer wg.Done()
		out := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), second, "shared-race-diff")
		rec2 = out.Result()
		b2 = out.Body.Bytes()
	}()
	wg.Wait()
	if atomic.LoadInt32(arrived) != 2 {
		t.Fatalf("expected both commits to observe a missing idempotency row, arrived=%d", atomic.LoadInt32(arrived))
	}

	created, conflict := 0, 0
	for i, rec := range []*http.Response{rec1, rec2} {
		body := b1
		if i == 1 {
			body = b2
		}
		switch rec.StatusCode {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			if errorMachineCode(body) != domain.MachineCodeIdempotencyConflict {
				t.Fatalf("loser machine_code=%s body=%s", errorMachineCode(body), string(body))
			}
			conflict++
		default:
			t.Fatalf("unexpected status %d body=%s", rec.StatusCode, string(body))
		}
	}
	if created != 1 || conflict != 1 {
		t.Fatalf("expected one 201 and one 409 idempotency_conflict, got created=%d conflict=%d b1=%s b2=%s", created, conflict, string(b1), string(b2))
	}
	if countERPEvents(t, env, tenantID) != 1 || countExternalLinks(t, env, tenantID) != 1 {
		t.Fatalf("expected exactly one event and one link, events=%d links=%d", countERPEvents(t, env, tenantID), countExternalLinks(t, env, tenantID))
	}
	if countIdempotencyRecords(t, env, tenantID, principal.ID, "shared-race-diff") != 1 {
		t.Fatal("expected exactly one idempotency record")
	}
	consumed, previewed := 0, 0
	for _, analysisID := range []uuid.UUID{first, second} {
		switch fetchAnalysisStatus(t, env, analysisID) {
		case domain.ImportAnalysisStatusConsumed:
			consumed++
		case domain.ImportAnalysisStatusPreviewed:
			previewed++
		default:
			t.Fatalf("unexpected analysis status %s", fetchAnalysisStatus(t, env, analysisID))
		}
	}
	if consumed != 1 || previewed != 1 {
		t.Fatalf("expected one CONSUMED and one PREVIEWED analysis, consumed=%d previewed=%d", consumed, previewed)
	}
}

func TestE4CreateCommitConcurrentIdempotentReplaySameBody(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router, erpSvc := seedCreateCommitReadyWithService(t, env, "RACE-SAME")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "RACE-SAME")
	arrived := armCreateCommitIdempotencyMissBarrier(t, erpSvc)

	var (
		wg   sync.WaitGroup
		rec1 *http.Response
		rec2 *http.Response
		b1   []byte
		b2   []byte
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		out := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "shared-race-same")
		rec1 = out.Result()
		b1 = out.Body.Bytes()
	}()
	go func() {
		defer wg.Done()
		out := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), analysisID, "shared-race-same")
		rec2 = out.Result()
		b2 = out.Body.Bytes()
	}()
	wg.Wait()
	if atomic.LoadInt32(arrived) != 2 {
		t.Fatalf("expected both commits to observe a missing idempotency row, arrived=%d", atomic.LoadInt32(arrived))
	}
	if rec1.StatusCode != http.StatusCreated || rec2.StatusCode != http.StatusCreated {
		t.Fatalf("expected both 201, got %d/%d b1=%s b2=%s", rec1.StatusCode, rec2.StatusCode, string(b1), string(b2))
	}
	if string(b1) != string(b2) {
		t.Fatalf("replay body mismatch %s vs %s", string(b1), string(b2))
	}
	if countERPEvents(t, env, tenantID) != 1 || countExternalLinks(t, env, tenantID) != 1 {
		t.Fatalf("same-body race must leave one event and one link, events=%d links=%d", countERPEvents(t, env, tenantID), countExternalLinks(t, env, tenantID))
	}
	if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusConsumed {
		t.Fatalf("analysis status=%s", fetchAnalysisStatus(t, env, analysisID))
	}
	if countIdempotencyRecords(t, env, tenantID, principal.ID, "shared-race-same") != 1 {
		t.Fatal("expected exactly one idempotency record")
	}
}

func TestE4CreateCommitRejectsTrailingJSON(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "TRAIL")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "TRAIL")
	beforeAnalyses := countERPAnalyses(t, env, tenantID)
	validPrefix := `{"analysis_id":"` + analysisID.String() + `"}`

	cases := []struct {
		name string
		body string
	}{
		{name: "second_object", body: validPrefix + `{"extra":1}`},
		{name: "second_scalar", body: validPrefix + `1`},
		{name: "corrupt_remainder", body: validPrefix + `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postERPCreateCommitRaw(t, router, tenantID, companyID, principal.ID, commitScopes(), []byte(tc.body), "key-trail-"+tc.name)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if countERPAnalyses(t, env, tenantID) != beforeAnalyses {
				t.Fatal("trailing JSON must not create an analysis")
			}
			if countERPEvents(t, env, tenantID) != 0 {
				t.Fatal("trailing JSON must not create an event")
			}
			if countExternalLinks(t, env, tenantID) != 0 {
				t.Fatal("trailing JSON must not create an external link")
			}
			if countIdempotencyRecords(t, env, tenantID, principal.ID, "key-trail-"+tc.name) != 0 {
				t.Fatal("trailing JSON must not store an idempotency record")
			}
			if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusPreviewed {
				t.Fatalf("analysis status=%s", fetchAnalysisStatus(t, env, analysisID))
			}
		})
	}
}

func TestE4CreateCommitAcceptsTrailingWhitespace(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, principal, router := seedCreateCommitReady(t, env, "WS")
	analysisID := previewCreateAnalysis(t, router, tenantID, companyID, principal.ID, "WS")
	body := []byte(`{"analysis_id":"` + analysisID.String() + `"}` + "  \n\t")
	rec := postERPCreateCommitRaw(t, router, tenantID, companyID, principal.ID, commitScopes(), body, "key-ws")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeCreateCommit(t, rec.Result(), rec.Body.Bytes())
	if out["creation_channel"] != domain.CreationChannelERP {
		t.Fatalf("creation_channel=%v", out["creation_channel"])
	}
	if countERPEvents(t, env, tenantID) != 1 || countExternalLinks(t, env, tenantID) != 1 {
		t.Fatal("valid commit with trailing whitespace must create one event and one link")
	}
}

func TestERPCreateCommitFeatureFlagDisabled(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "e4-flag")
	router := newERPPreviewRouter(t, env, config.Config{RfxErpIntegrationEnabled: false})
	rec := postERPCreateCommit(t, router, tenantID, companyID, principal.ID, commitScopes(), uuid.New(), "key-flag")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if countERPEvents(t, env, tenantID) != 0 || countERPAnalyses(t, env, tenantID) != 0 {
		t.Fatal("disabled flag must write zero records")
	}
}
