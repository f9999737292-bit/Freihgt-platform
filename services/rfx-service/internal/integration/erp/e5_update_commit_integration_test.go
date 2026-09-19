//go:build integration

package erp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

func updateDraftPayload(title, lotNumber, lotName string) string {
	return `{
		"schema_version":"BINTRANS_RFX_ERP_JSON_V1",
		"requested_operation":"UPDATE_DRAFT",
		"event":{"type":"SPOT_RFQ","title":"` + title + `","currency":"USD_EXT","timezone":"UTC_EXT"},
		"lots":[{"lot_number":"` + lotNumber + `","name":"` + lotName + `"}],
		"questionnaire":{"sections":[{"section_code":"S1","title":"Section 1"}]}
	}`
}

func seedUpdateCommitReady(t *testing.T, env *testEnv, clientSuffix string) (tenantID, companyID, eventID uuid.UUID, principal *domain.IntegrationPrincipal, router http.Handler) {
	t.Helper()
	tenantID, companyID, eventID, principal, router, _ = seedUpdateCommitReadyWithService(t, env, clientSuffix)
	return tenantID, companyID, eventID, principal, router
}

func seedUpdateCommitReadyWithService(t *testing.T, env *testEnv, clientSuffix string) (tenantID, companyID, eventID uuid.UUID, principal *domain.IntegrationPrincipal, router http.Handler, erpSvc *service.ErpIntegrationService) {
	t.Helper()
	tenantID, companyID = seedTenantCompany(t, env)
	seedPlatformMappingSet(t, env, tenantID, "CURRENCY", 1, "USD_EXT", "USD")
	seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 1, "UTC_EXT", "UTC")
	eventID = seedDraftRfxEvent(t, env, tenantID, companyID)
	principal = seedIntegrationPrincipal(t, env, tenantID, companyID, "e5-"+clientSuffix)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftRead)
	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftCommit)
	router, erpSvc = newERPPreviewRouterAndService(t, env, enabledERPIntegrationConfig())
	return tenantID, companyID, eventID, principal, router, erpSvc
}

func previewUpdateAnalysis(t *testing.T, router http.Handler, eventID, tenantID, companyID, principalID uuid.UUID, title, lotNumber, lotName string) uuid.UUID {
	t.Helper()
	rec := postERPUpdatePreview(t, router, eventID, tenantID, companyID, principalID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftRead}, []byte(updateDraftPayload(title, lotNumber, lotName)))
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

func decodeUpdateCommit(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode commit: %v body=%s", err, string(body))
	}
	return out
}

func countEventLots(t *testing.T, env *testEnv, eventID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_lots WHERE rfx_event_id = $1 AND deleted_at IS NULL
	`, eventID).Scan(&count); err != nil {
		t.Fatalf("count lots: %v", err)
	}
	return count
}

func countQuestionnaireSections(t *testing.T, env *testEnv, eventID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM rfx.rfx_sections s
		JOIN rfx.rfx_versions v ON v.id = s.rfx_version_id
		WHERE v.rfx_event_id = $1 AND s.deleted_at IS NULL
	`, eventID).Scan(&count); err != nil {
		t.Fatalf("count sections: %v", err)
	}
	return count
}

func countCarrierResponses(t *testing.T, env *testEnv, eventID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_responses WHERE rfx_event_id = $1
	`, eventID).Scan(&count); err != nil {
		t.Fatalf("count responses: %v", err)
	}
	return count
}

func countScoreModels(t *testing.T, env *testEnv, eventID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM rfx.rfx_score_models sm
		JOIN rfx.rfx_versions v ON v.id = sm.rfx_version_id
		WHERE v.rfx_event_id = $1
	`, eventID).Scan(&count); err != nil {
		t.Fatalf("count score models: %v", err)
	}
	return count
}

func countAwards(t *testing.T, env *testEnv, eventID uuid.UUID) int {
	t.Helper()
	var count int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM rfx.rfx_awards WHERE rfx_event_id = $1
	`, eventID).Scan(&count); err != nil {
		t.Fatalf("count awards: %v", err)
	}
	return count
}

func fetchEventTitleStatus(t *testing.T, env *testEnv, eventID uuid.UUID) (title, status string) {
	t.Helper()
	if err := env.pool.QueryRow(context.Background(), `
		SELECT title, status FROM rfx.rfx_events WHERE id = $1
	`, eventID).Scan(&title, &status); err != nil {
		t.Fatalf("fetch event: %v", err)
	}
	return title, status
}

func bumpEventRowVersion(t *testing.T, env *testEnv, eventID uuid.UUID) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_events SET version = version + 1, updated_at = now() WHERE id = $1
	`, eventID); err != nil {
		t.Fatalf("bump event version: %v", err)
	}
}

func bumpDraftRowVersion(t *testing.T, env *testEnv, eventID, tenantID uuid.UUID) {
	t.Helper()
	qRepo := repository.NewQuestionnaireRepository(env.pool)
	draft, err := qRepo.GetActiveDraftVersion(context.Background(), tenantID, eventID)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if _, err := qRepo.TouchDraftVersion(context.Background(), draft.ID, tenantID, draft.Version); err != nil {
		t.Fatalf("touch draft: %v", err)
	}
}

func insertLot(t *testing.T, env *testEnv, eventID, tenantID uuid.UUID, lotNumber, name string) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(), `
		INSERT INTO rfx.rfx_lots (tenant_id, rfx_event_id, lot_number, name, status)
		VALUES ($1, $2, $3, $4, 'ACTIVE')
	`, tenantID, eventID, lotNumber, name); err != nil {
		t.Fatalf("insert lot: %v", err)
	}
}

func TestE7P2INT150UpdateCommitAppliesGraph(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT150")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Update Applied", "L1", "Lot One")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-150")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := decodeUpdateCommit(t, rec.Body.Bytes())
	if out["rfx_event_id"] != eventID.String() {
		t.Fatalf("rfx_event_id=%v", out["rfx_event_id"])
	}
	if _, err := time.Parse(time.RFC3339Nano, out["applied_at"].(string)); err != nil {
		if _, err2 := time.Parse(time.RFC3339, out["applied_at"].(string)); err2 != nil {
			t.Fatalf("applied_at=%v", out["applied_at"])
		}
	}
	title, status := fetchEventTitleStatus(t, env, eventID)
	if title != "ERP Update Applied" || status != domain.RfxStatusDraft {
		t.Fatalf("title=%s status=%s", title, status)
	}
	if countEventLots(t, env, eventID) != 1 {
		t.Fatalf("lots=%d", countEventLots(t, env, eventID))
	}
	if countQuestionnaireSections(t, env, eventID) != 1 {
		t.Fatalf("sections=%d", countQuestionnaireSections(t, env, eventID))
	}
	if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusConsumed {
		t.Fatalf("analysis status=%s", fetchAnalysisStatus(t, env, analysisID))
	}
}

func TestE7P2INT151StaleEventRowVersion409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT151")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Stale Event", "L1", "Lot")
	bumpEventRowVersion(t, env, eventID)
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-151")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeStaleTarget {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
	if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusPreviewed {
		t.Fatal("stale event must not consume analysis")
	}
}

func TestE7P2INT152StaleDraftRowVersion409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT152")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Stale Draft", "L1", "Lot")
	bumpDraftRowVersion(t, env, eventID, tenantID)
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-152")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeProposalRevalidation {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
}

func TestE7P2INT153CanonicalHashTampered409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT153")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Hash", "L1", "Lot")
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin tamper tx: %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `SET LOCAL session_replication_role = replica`); err != nil {
		t.Fatalf("disable immutable trigger: %v", err)
	}
	if _, err := tx.Exec(context.Background(), `
		UPDATE rfx.rfx_import_analyses
		SET canonical_hash = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
		WHERE id = $1
	`, analysisID); err != nil {
		t.Fatalf("tamper hash: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit tamper tx: %v", err)
	}
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-153")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeCanonicalHashMismatch {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
}

func TestE7P2INT154ConsumedAnalysisNewKey409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT154")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Consumed", "L1", "Lot")
	first := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-154-a")
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-154-b")
	if second.Code != http.StatusConflict || errorMachineCode(second.Body.Bytes()) != domain.MachineCodeAnalysisAlreadyConsumed {
		t.Fatalf("status=%d code=%s body=%s", second.Code, errorMachineCode(second.Body.Bytes()), second.Body.String())
	}
}

func TestE7P2INT155ExpiredAnalysis409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT155")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Expired", "L1", "Lot")
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_import_analyses SET expires_at = now() - interval '1 minute' WHERE id = $1
	`, analysisID); err != nil {
		t.Fatalf("expire analysis: %v", err)
	}
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-155")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeAnalysisExpired {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
}

func TestE7P2INT156PrincipalBindingMismatch409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principalA, router := seedUpdateCommitReady(t, env, "INT156")
	principalB := seedIntegrationPrincipal(t, env, tenantID, companyID, "e5-int156-b")
	grantScope(t, env, tenantID, principalB.ID, domain.ScopeDraftRead)
	grantScope(t, env, tenantID, principalB.ID, domain.ScopeDraftCommit)
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principalA.ID, "ERP Binding", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principalB.ID, updateCommitScopes(), analysisID, "key-156")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeActorBindingDenied {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
	if countEventLots(t, env, eventID) != 0 {
		t.Fatal("binding denial must not apply lots")
	}
}

func TestE7P2INT157UpdateIdempotentReplay(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT157")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Replay", "L1", "Lot")
	first := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-157")
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-157")
	if second.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", second.Code, second.Body.String())
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("replay body mismatch %s vs %s", first.Body.String(), second.Body.String())
	}
	if countEventLots(t, env, eventID) != 1 {
		t.Fatal("replay must not apply a second lot graph")
	}
}

func TestE7P2INT158ConcurrentUpdateOneWins(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT158")
	first := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Concurrent A", "L1", "Lot A")
	second := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Concurrent B", "L2", "Lot B")
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
		out := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), first, "key-158-a")
		rec1 = out.Result()
		b1 = out.Body.Bytes()
	}()
	go func() {
		defer wg.Done()
		out := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), second, "key-158-b")
		rec2 = out.Result()
		b2 = out.Body.Bytes()
	}()
	wg.Wait()
	ok, stale := 0, 0
	for i, rec := range []*http.Response{rec1, rec2} {
		body := b1
		if i == 1 {
			body = b2
		}
		switch rec.StatusCode {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			code := errorMachineCode(body)
			if code != domain.MachineCodeStaleTarget && code != domain.MachineCodeProposalRevalidation {
				t.Fatalf("loser machine_code=%s body=%s", code, string(body))
			}
			stale++
		default:
			t.Fatalf("unexpected status %d body=%s", rec.StatusCode, string(body))
		}
	}
	if ok != 1 || stale != 1 {
		t.Fatalf("expected one 200 and one stale 409, got ok=%d stale=%d b1=%s b2=%s", ok, stale, string(b1), string(b2))
	}
}

func TestE7P2INT159RollbackNoPartialWrites(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router, erpSvc := seedUpdateCommitReadyWithService(t, env, "INT159")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Rollback", "L1", "Lot")
	erpSvc.SetAfterUpdateCommitApply(func() error {
		return errors.New("forced update apply failure")
	})
	t.Cleanup(func() { erpSvc.SetAfterUpdateCommitApply(nil) })
	beforeLots := countEventLots(t, env, eventID)
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-159")
	if rec.Code == http.StatusOK {
		t.Fatal("forced failure must not succeed")
	}
	if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusPreviewed {
		t.Fatal("failed commit must leave analysis PREVIEWED")
	}
	if countEventLots(t, env, eventID) != beforeLots {
		t.Fatal("failed commit must not persist lots")
	}
	if countIdempotencyRecords(t, env, tenantID, principal.ID, "key-159") != 0 {
		t.Fatal("failed commit must not store idempotency")
	}
	title, _ := fetchEventTitleStatus(t, env, eventID)
	if title == "ERP Rollback" {
		t.Fatal("failed commit must roll back event title")
	}
}

func TestE7P2INT177UpdateCommitDoesNotPublish(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT177U")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Stay Draft", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-177u")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	_, status := fetchEventTitleStatus(t, env, eventID)
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
		t.Fatal("UPDATE commit must not publish")
	}
}

func TestE7P2INT178NoCarrierMutation(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT178")
	before := countCarrierResponses(t, env, eventID)
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Carrier Safe", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-178")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if countCarrierResponses(t, env, eventID) != before {
		t.Fatal("UPDATE commit must not mutate carrier responses")
	}
}

func TestE7P2INT179NoScoringAwardMutation(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT179")
	beforeScores := countScoreModels(t, env, eventID)
	beforeAwards := countAwards(t, env, eventID)
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Score Safe", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-179")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if countScoreModels(t, env, eventID) != beforeScores || countAwards(t, env, eventID) != beforeAwards {
		t.Fatal("UPDATE commit must not mutate scoring or awards")
	}
}

func TestE7P2INT181AuditUpdateCommit(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT181")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Audit", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-181")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	auditRepo := repository.NewAuditRepository(env.pool)
	events, err := auditRepo.ListByEntity(context.Background(), tenantID, "rfx_event", eventID, 20)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Action != domain.ERPUpdateCommitAuditAction {
			continue
		}
		found = true
		if ev.ActorUserID != nil {
			t.Fatal("ERP audit must not synthesize a human actor_id")
		}
		if ev.Metadata["actor_kind"] != domain.AuditActorKindIntegration {
			t.Fatalf("actor_kind=%v", ev.Metadata["actor_kind"])
		}
	}
	if !found {
		t.Fatal("expected rfx.erp.draft.updated.v1")
	}
}

func TestE7P2INT183ERPUICoexistenceStale(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT183")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP UI", "L1", "Lot")
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_events SET title = 'UI Edit', version = version + 1, updated_at = now() WHERE id = $1
	`, eventID); err != nil {
		t.Fatalf("ui edit: %v", err)
	}
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-183")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeStaleTarget {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
}

func TestE7P2INT184ERPXLSXCoexistenceStale(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT184")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP XLSX", "L1", "Lot")
	insertLot(t, env, eventID, tenantID, "XLSX-1", "XLSX Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-184")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeStaleTarget {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
}

func TestE7P2INT191UpdatePrincipalBindingRegression(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principalA, router := seedUpdateCommitReady(t, env, "INT191U")
	principalB := seedIntegrationPrincipal(t, env, tenantID, companyID, "e5-int191-b")
	grantScope(t, env, tenantID, principalB.ID, domain.ScopeDraftRead)
	grantScope(t, env, tenantID, principalB.ID, domain.ScopeDraftCommit)
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principalA.ID, "ERP 191", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principalB.ID, updateCommitScopes(), analysisID, "key-191u")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeActorBindingDenied {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
}

func TestE7P2INT193UpdateStaleMappingContext409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "INT193U")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Mapping", "L1", "Lot")
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE rfx.rfx_reference_mapping_sets SET status = $1 WHERE tenant_id IS NULL
	`, domain.ReferenceMappingSetStatusRetired); err != nil {
		t.Fatalf("retire mapping sets: %v", err)
	}
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-193u")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeStaleMappingContext {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
}

func TestE5UpdateCommitMissingScope403(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "SCOPE")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Scope", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, []string{domain.ScopeDraftRead}, analysisID, "key-scope")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE5UpdateCommitFeatureFlagDisabled(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	eventID := seedDraftRfxEvent(t, env, tenantID, companyID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "e5-flag")
	router := newERPPreviewRouter(t, env, config.Config{RfxErpIntegrationEnabled: false})
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), uuid.New(), "key-flag")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestE5UpdateCommitRejectsTrailingJSON(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "TRAIL")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Trail", "L1", "Lot")
	body := []byte(`{"analysis_id":"` + analysisID.String() + `"}{"extra":1}`)
	rec := postERPUpdateCommitRaw(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), body, "key-trail")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusPreviewed {
		t.Fatal("trailing JSON must not consume analysis")
	}
}

func TestE5UpdatePreviewPersistsBaselineTokens(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "BASELINE")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Tokens", "L1", "Lot")
	payload := fetchAnalysisCanonicalJSON(t, env, analysisID)
	var stored map[string]any
	if err := json.Unmarshal(payload, &stored); err != nil {
		t.Fatalf("decode canonical: %v", err)
	}
	if stored["event_row_version"] == nil || stored["draft_row_version"] == nil || stored["baseline_lots_fingerprint"] == nil {
		t.Fatalf("baseline tokens missing: %s", string(payload))
	}
	if stored["event_row_version"] == stored["draft_row_version"] && stored["event_row_version"] == float64(0) {
		t.Fatal("baseline versions must not collapse to zero")
	}
}

func TestE5UpdateConcurrentSameKeyReplay(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router, erpSvc := seedUpdateCommitReadyWithService(t, env, "RACE-SAME")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP Race Same", "L1", "Lot")
	var arrived int32
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	go func() {
		<-ready
		<-ready
		close(release)
	}()
	erpSvc.SetAfterUpdateCommitIdempotencyMiss(func() {
		atomic.AddInt32(&arrived, 1)
		ready <- struct{}{}
		select {
		case <-release:
		case <-time.After(10 * time.Second):
			t.Error("timed out waiting for concurrent UPDATE commit peer")
		}
	})
	t.Cleanup(func() { erpSvc.SetAfterUpdateCommitIdempotencyMiss(nil) })

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
		out := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "shared-e5-same")
		rec1 = out.Result()
		b1 = out.Body.Bytes()
	}()
	go func() {
		defer wg.Done()
		out := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "shared-e5-same")
		rec2 = out.Result()
		b2 = out.Body.Bytes()
	}()
	wg.Wait()
	if atomic.LoadInt32(&arrived) != 2 {
		t.Fatalf("arrived=%d", arrived)
	}
	if rec1.StatusCode != http.StatusOK || rec2.StatusCode != http.StatusOK {
		t.Fatalf("expected both 200, got %d/%d b1=%s b2=%s", rec1.StatusCode, rec2.StatusCode, string(b1), string(b2))
	}
	if string(b1) != string(b2) {
		t.Fatalf("replay body mismatch %s vs %s", string(b1), string(b2))
	}
	if countEventLots(t, env, eventID) != 1 {
		t.Fatal("same-body race must apply lots once")
	}
}
