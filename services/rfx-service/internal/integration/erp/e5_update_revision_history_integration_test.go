//go:build integration

package erp

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"

	stderrors "errors"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const e4SeedPayloadHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func updateDraftPayloadWithExternal(title, lotNumber, lotName, system, objectID, revision string) string {
	external := `"external":{"system":"` + system + `","object_id":"` + objectID + `"`
	if revision != "" {
		external += `,"revision":"` + revision + `"`
	}
	external += `},`
	return `{
		"schema_version":"BINTRANS_RFX_ERP_JSON_V1",
		"requested_operation":"UPDATE_DRAFT",
		` + external + `
		"event":{"type":"SPOT_RFQ","title":"` + title + `","currency":"USD_EXT","timezone":"UTC_EXT"},
		"lots":[{"lot_number":"` + lotNumber + `","name":"` + lotName + `"}]
	}`
}

func previewUpdateAnalysisWithExternal(t *testing.T, router http.Handler, eventID, tenantID, companyID, principalID uuid.UUID, title, lotNumber, lotName, system, objectID, revision string) uuid.UUID {
	t.Helper()
	rec := postERPUpdatePreview(t, router, eventID, tenantID, companyID, principalID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftRead},
		[]byte(updateDraftPayloadWithExternal(title, lotNumber, lotName, system, objectID, revision)))
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

func seedE4ExternalLink(t *testing.T, env *testEnv, tenantID, principalID, eventID uuid.UUID, system, objectID, revision, payloadHash string) uuid.UUID {
	t.Helper()
	link, err := env.externalLinkRepo.InsertLink(context.Background(), domain.ExternalObjectLink{
		TenantID:               tenantID,
		IntegrationPrincipalID: principalID,
		ExternalSystem:         system,
		ExternalObjectType:     domain.ExternalObjectTypeRfxEvent,
		ExternalObjectID:       objectID,
		ExternalVersion:        revision,
		ExternalRevision:       revision,
		PayloadHash:            payloadHash,
		RfxEventID:             eventID,
	})
	if err != nil {
		t.Fatalf("seed E4 link: %v", err)
	}
	return link.ID
}

func fetchEventLink(t *testing.T, env *testEnv, tenantID, principalID, eventID uuid.UUID) (id uuid.UUID, revision, payloadHash string, found bool) {
	t.Helper()
	link, err := env.externalLinkRepo.GetByRfxEventID(context.Background(), tenantID, principalID, eventID)
	if err != nil {
		var appErr *apperrors.AppError
		if stderrors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return uuid.Nil, "", "", false
		}
		t.Fatalf("get link: %v", err)
	}
	return link.ID, link.ExternalRevision, link.PayloadHash, true
}

func listLinkRevisions(t *testing.T, env *testEnv, tenantID, linkID uuid.UUID) []domain.ExternalObjectLinkRevision {
	t.Helper()
	rows, err := env.externalLinkRepo.ListRevisions(context.Background(), tenantID, linkID)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	return rows
}

func decodeErrorEnvelope(t *testing.T, body []byte) (code, field, machine string) {
	t.Helper()
	var env struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error: %v body=%s", err, string(body))
	}
	field, _ = env.Error.Details["field"].(string)
	machine, _ = env.Error.Details["machine_code"].(string)
	return env.Error.Code, field, machine
}

func TestE5ExistingLinkUpdatePersistsPreviousAndNewRevision(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "REV-OK")
	linkID := seedE4ExternalLink(t, env, tenantID, principal.ID, eventID, "SAP", "OBJ-REV-1", "1", e4SeedPayloadHash)
	if got := listLinkRevisions(t, env, tenantID, linkID); len(got) != 0 {
		t.Fatalf("E4 CREATE leftover must have empty history, got %d", len(got))
	}
	analysisID := previewUpdateAnalysisWithExternal(t, router, eventID, tenantID, companyID, principal.ID, "ERP Rev 2", "L1", "Lot", "SAP", "OBJ-REV-1", "2")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-rev-ok")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	gotID, revision, newHash, found := fetchEventLink(t, env, tenantID, principal.ID, eventID)
	if !found || gotID != linkID {
		t.Fatal("UPDATE must keep the same link and rfx_event_id binding")
	}
	if revision != "2" {
		t.Fatalf("link revision=%s", revision)
	}
	if newHash == "" || newHash == e4SeedPayloadHash {
		t.Fatal("payload_hash must change on accepted UPDATE")
	}
	history := listLinkRevisions(t, env, tenantID, linkID)
	if len(history) != 2 {
		t.Fatalf("history rows=%d", len(history))
	}
	byRev := map[string]string{}
	for _, row := range history {
		byRev[row.ExternalRevision] = row.PayloadHash
	}
	if byRev["1"] != e4SeedPayloadHash {
		t.Fatalf("history[1] hash=%s", byRev["1"])
	}
	if byRev["2"] != newHash {
		t.Fatalf("history[2] hash=%s want %s", byRev["2"], newHash)
	}
	if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusConsumed {
		t.Fatal("analysis must be consumed")
	}
}

func TestE5ExistingLinkRejectsMissingExternalAndRepeatedRevision(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "REV-REJ")
	linkID := seedE4ExternalLink(t, env, tenantID, principal.ID, eventID, "SAP", "OBJ-REV-REJ", "1", e4SeedPayloadHash)
	beforeAnalyses := countERPAnalyses(t, env, tenantID)
	titleBefore, _ := fetchEventTitleStatus(t, env, eventID)

	cases := []struct {
		name  string
		body  string
		field string
	}{
		{"missing_external", updateDraftPayload("No External", "L1", "Lot"), "external.revision"},
		{"missing_revision", updateDraftPayloadWithExternal("No Rev", "L1", "Lot", "SAP", "OBJ-REV-REJ", ""), "external.revision"},
		{"repeat_current", updateDraftPayloadWithExternal("Repeat 1", "L1", "Lot", "SAP", "OBJ-REV-REJ", "1"), "external.revision"},
		{"change_system", updateDraftPayloadWithExternal("Change Sys", "L1", "Lot", "ORACLE", "OBJ-REV-REJ", "2"), "external.system"},
		{"change_object", updateDraftPayloadWithExternal("Change Obj", "L1", "Lot", "SAP", "OBJ-OTHER", "2"), "external.object_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postERPUpdatePreview(t, router, eventID, tenantID, companyID, principal.ID,
				[]string{domain.ScopeDraftPreview, domain.ScopeDraftRead}, []byte(tc.body))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			code, field, machine := decodeErrorEnvelope(t, rec.Body.Bytes())
			if code != string(apperrors.CodeValidation) || field != tc.field {
				t.Fatalf("code=%s field=%s body=%s", code, field, rec.Body.String())
			}
			if machine != "" {
				t.Fatalf("new machine_code not allowed: %s", machine)
			}
		})
	}

	ok := previewUpdateAnalysisWithExternal(t, router, eventID, tenantID, companyID, principal.ID, "ERP Rev 2", "L1", "Lot", "SAP", "OBJ-REV-REJ", "2")
	if rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), ok, "key-rev-rej-ok"); rec.Code != http.StatusOK {
		t.Fatalf("setup second revision: %d %s", rec.Code, rec.Body.String())
	}
	replayUsed := postERPUpdatePreview(t, router, eventID, tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftRead},
		[]byte(updateDraftPayloadWithExternal("Reuse 1", "L2", "Lot", "SAP", "OBJ-REV-REJ", "1")))
	if replayUsed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("used revision status=%d body=%s", replayUsed.Code, replayUsed.Body.String())
	}
	code, field, machine := decodeErrorEnvelope(t, replayUsed.Body.Bytes())
	if code != string(apperrors.CodeValidation) || field != "external.revision" || machine != "" {
		t.Fatalf("used revision code=%s field=%s machine=%s", code, field, machine)
	}

	if countERPAnalyses(t, env, tenantID) != beforeAnalyses+1 {
		t.Fatal("rejected previews must not persist analysis")
	}
	titleAfter, _ := fetchEventTitleStatus(t, env, eventID)
	if titleAfter == "No External" || titleAfter == titleBefore && titleAfter == "No External" {
		t.Fatal("rejected preview must not change event")
	}
	gotID, revision, hash, found := fetchEventLink(t, env, tenantID, principal.ID, eventID)
	if !found || gotID != linkID || revision != "2" {
		t.Fatalf("link drifted on reject path: found=%v rev=%s", found, revision)
	}
	if hash == "" {
		t.Fatal("link hash missing")
	}
}

func TestE5ExistingLinkCommitStaleAfterLinkDrift(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "REV-DRIFT")
	linkID := seedE4ExternalLink(t, env, tenantID, principal.ID, eventID, "SAP", "OBJ-DRIFT", "1", e4SeedPayloadHash)
	analysisID := previewUpdateAnalysisWithExternal(t, router, eventID, tenantID, companyID, principal.ID, "ERP Drift", "L1", "Lot", "SAP", "OBJ-DRIFT", "2")
	if _, err := env.externalLinkRepo.UpdateLinkMetadataInPlace(
		context.Background(), linkID, tenantID, principal.ID, eventID, "9",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	); err != nil {
		t.Fatalf("drift link: %v", err)
	}
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-rev-drift")
	if rec.Code != http.StatusConflict || errorMachineCode(rec.Body.Bytes()) != domain.MachineCodeStaleTarget {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, errorMachineCode(rec.Body.Bytes()), rec.Body.String())
	}
	if fetchAnalysisStatus(t, env, analysisID) != domain.ImportAnalysisStatusPreviewed {
		t.Fatal("stale link must leave analysis PREVIEWED")
	}
	if history := listLinkRevisions(t, env, tenantID, linkID); len(history) != 0 {
		t.Fatalf("drift reject must not write history, got %d", len(history))
	}
	_, revision, _, _ := fetchEventLink(t, env, tenantID, principal.ID, eventID)
	if revision != "9" {
		t.Fatalf("link revision after reject=%s", revision)
	}
}

func TestE5UIEventWithoutLinkAndFirstBind(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "REV-UI")
	analysisID := previewUpdateAnalysis(t, router, eventID, tenantID, companyID, principal.ID, "ERP UI No Link", "L1", "Lot")
	rec := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-rev-ui")
	if rec.Code != http.StatusOK {
		t.Fatalf("ui update status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, _, _, found := fetchEventLink(t, env, tenantID, principal.ID, eventID); found {
		t.Fatal("UI UPDATE without external must not create a link")
	}

	bindID := previewUpdateAnalysisWithExternal(t, router, eventID, tenantID, companyID, principal.ID, "ERP First Bind", "L2", "Lot 2", "SAP", "OBJ-FIRST", "1")
	bind := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), bindID, "key-rev-bind")
	if bind.Code != http.StatusOK {
		t.Fatalf("first bind status=%d body=%s", bind.Code, bind.Body.String())
	}
	linkID, revision, hash, found := fetchEventLink(t, env, tenantID, principal.ID, eventID)
	if !found || revision != "1" || hash == "" {
		t.Fatalf("first bind link found=%v rev=%s", found, revision)
	}
	history := listLinkRevisions(t, env, tenantID, linkID)
	if len(history) != 1 || history[0].ExternalRevision != "1" || history[0].PayloadHash != hash {
		t.Fatalf("first bind history=%+v", history)
	}
}

func TestE5ExistingLinkSameKeyReplayAndParallelUpdate(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID, eventID, principal, router := seedUpdateCommitReady(t, env, "REV-RACE")
	linkID := seedE4ExternalLink(t, env, tenantID, principal.ID, eventID, "SAP", "OBJ-RACE", "1", e4SeedPayloadHash)
	analysisID := previewUpdateAnalysisWithExternal(t, router, eventID, tenantID, companyID, principal.ID, "ERP Replay Rev", "L1", "Lot", "SAP", "OBJ-RACE", "2")
	first := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-rev-replay")
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), analysisID, "key-rev-replay")
	if second.Code != http.StatusOK || first.Body.String() != second.Body.String() {
		t.Fatalf("replay status=%d body=%s vs %s", second.Code, second.Body.String(), first.Body.String())
	}
	if history := listLinkRevisions(t, env, tenantID, linkID); len(history) != 2 {
		t.Fatalf("replay must not add history rows, got %d", len(history))
	}

	previewA := previewUpdateAnalysisWithExternal(t, router, eventID, tenantID, companyID, principal.ID, "ERP Par A", "LA", "Lot A", "SAP", "OBJ-RACE", "3")
	previewB := previewUpdateAnalysisWithExternal(t, router, eventID, tenantID, companyID, principal.ID, "ERP Par B", "LB", "Lot B", "SAP", "OBJ-RACE", "4")
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
		out := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), previewA, "key-rev-par-a")
		rec1 = out.Result()
		b1 = out.Body.Bytes()
	}()
	go func() {
		defer wg.Done()
		out := postERPUpdateCommit(t, router, eventID, tenantID, companyID, principal.ID, updateCommitScopes(), previewB, "key-rev-par-b")
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
			if errorMachineCode(body) != domain.MachineCodeStaleTarget && errorMachineCode(body) != domain.MachineCodeProposalRevalidation {
				t.Fatalf("loser code=%s body=%s", errorMachineCode(body), string(body))
			}
			stale++
		default:
			t.Fatalf("unexpected status %d body=%s", rec.StatusCode, string(body))
		}
	}
	if ok != 1 || stale != 1 {
		t.Fatalf("expected one winner, got ok=%d stale=%d", ok, stale)
	}
	history := listLinkRevisions(t, env, tenantID, linkID)
	if len(history) != 3 {
		t.Fatalf("parallel history rows=%d want 3 (1,2,winner)", len(history))
	}
	seen := map[string]int{}
	for _, row := range history {
		seen[row.ExternalRevision]++
	}
	if seen["1"] != 1 || seen["2"] != 1 || (seen["3"]+seen["4"]) != 1 {
		t.Fatalf("history revisions=%v", seen)
	}
	if _, _, _, found := fetchEventLink(t, env, tenantID, principal.ID, eventID); !found {
		t.Fatal("link missing after parallel update")
	}
}
