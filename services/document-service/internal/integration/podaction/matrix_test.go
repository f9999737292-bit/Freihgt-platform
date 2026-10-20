//go:build integration

package podaction

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

func TestPOD_BIND_01(t *testing.T) {
	fix := seedAction(t, true)
	if fix.operating == fix.shipmentTenant {
		t.Fatal("cross-shipper fixture collapsed")
	}
	done := finish(t, fix.input("bind-01"))
	var docTenant uuid.UUID
	var related string
	var relatedID uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `
		SELECT tenant_id, related_entity_type, related_entity_id
		FROM documents.documents WHERE id=$1`, done.DocumentID).Scan(&docTenant, &related, &relatedID); err != nil {
		t.Fatal(err)
	}
	if docTenant != fix.operating || related != "POD" && related != "SHIPMENT" || relatedID != fix.shipment {
		t.Fatalf("document tenant=%s related=%s/%s", docTenant, related, relatedID)
	}
	if related != "SHIPMENT" {
		t.Fatalf("related type %s", related)
	}
	rows, err := env.svc.GetEvidence(env.ctx, fix.operating, fix.execution, fix.action)
	if err != nil || len(rows) != 1 || rows[0].DocumentID != done.DocumentID || rows[0].ShipmentTenantID != fix.shipmentTenant || rows[0].DocumentFileID != done.FileID {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	var sameTenantShipment int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM transport.shipments WHERE id=$1 AND tenant_id=$2`, fix.shipment, fix.operating).Scan(&sameTenantShipment); err != nil {
		t.Fatal(err)
	}
	if sameTenantShipment != 0 {
		t.Fatal("shipment ownership moved into the carrier tenant")
	}
	var provider string
	var bucket *string
	if err := env.pool.QueryRow(env.ctx, `SELECT storage_provider, bucket_name FROM documents.document_files WHERE id=$1`, done.FileID).Scan(&provider, &bucket); err != nil {
		t.Fatal(err)
	}
	if provider != "local" || bucket != nil {
		t.Fatalf("local file metadata provider=%s bucket=%v", provider, bucket)
	}
}

func TestPOD_BIND_02(t *testing.T) {
	fix := seedAction(t, true)
	pickup := extraAction(t, fix, "PICKUP", 1)
	in := pickup.input("bind-02")
	in.ActionType = "PICKUP"
	if _, err := env.svc.CreateUploadIntent(env.ctx, in); codeOf(err) != apperrors.CodeValidation {
		t.Fatalf("pickup command: %v", err)
	}
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM documents.pod_delivery_action_upload_intent WHERE idempotency_key=$1`, "bind-02").Scan(&n); err != nil || n != 0 {
		t.Fatalf("pickup command inserted %d err=%v", n, err)
	}
}

func TestPOD_BIND_03(t *testing.T) {
	fix := seedAction(t, true)
	done := finish(t, fix.input("bind-03"))
	rows, err := env.svc.GetEvidence(env.ctx, fix.operating, uuid.New(), uuid.New())
	if err != nil || len(rows) != 0 {
		t.Fatalf("unknown scope %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.DocumentID == done.DocumentID {
			t.Fatal("unknown scope leaked document id")
		}
	}
}

func TestPOD_BIND_05(t *testing.T) {
	fix := seedAction(t, true)
	in := fix.input("bind-05")
	intent, err := env.svc.CreateUploadIntent(env.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	other := uuid.New()
	in.ExecutionID = other
	if _, err := env.svc.CreateUploadIntent(env.ctx, in); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("replay: %v", err)
	}
	err = env.svc.UploadContent(env.ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("x")), domain.SubjectConfirmation{ExecutionID: &other})
	if codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("content: %v", err)
	}
}

func TestPOD_BIND_06(t *testing.T) {
	fix := seedAction(t, true)
	in := fix.input("bind-06")
	intent, err := env.svc.CreateUploadIntent(env.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.svc.UploadContent(env.ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("x")), domain.SubjectConfirmation{}); err != nil {
		t.Fatal(err)
	}
	other := uuid.New()
	if _, err := env.svc.CompleteUpload(env.ctx, intent.UploadID, "", domain.SubjectConfirmation{ShipmentID: &other}); codeOf(err) != apperrors.CodeConflict || countEvidence(t, intent.DocumentID) != 0 {
		t.Fatalf("complete: %v", err)
	}
}

func TestPOD_BIND_07(t *testing.T) {
	fix := seedAction(t, true)
	done := finish(t, fix.input("bind-07"))
	foreign := uuid.New()
	rows, err := env.svc.GetEvidence(env.ctx, foreign, fix.execution, fix.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("foreign read %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.DocumentID == done.DocumentID {
			t.Fatal("foreign operating tenant leaked document id")
		}
	}
}

func TestPOD_BIND_08(t *testing.T) {
	fix := seedAction(t, true)
	in := fix.input("bind-08")
	if _, err := env.svc.CreateUploadIntent(env.ctx, in); err != nil {
		t.Fatal(err)
	}
	in.ShipmentTenantID = uuid.New()
	if _, err := env.svc.CreateUploadIntent(env.ctx, in); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("replay pin: %v", err)
	}
}

func TestPOD_BIND_09(t *testing.T) {
	fix := seedAction(t, true)
	in := fix.input("bind-09")
	if _, err := env.svc.CreateUploadIntent(env.ctx, in); err != nil {
		t.Fatal(err)
	}
	in.AuthorDriverID = uuid.New()
	if _, err := env.svc.CreateUploadIntent(env.ctx, in); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("driver: %v", err)
	}
}

func TestPOD_BIND_10(t *testing.T) {
	fix := seedAction(t, true)
	in := fix.input("bind-10")
	first, err := env.svc.CreateUploadIntent(env.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.svc.CreateUploadIntent(env.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.UploadID != second.UploadID || first.DocumentID != second.DocumentID {
		t.Fatal("replay changed identity")
	}
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM documents.pod_delivery_action_upload_intent WHERE document_tenant_id=$1 AND idempotency_key=$2`, fix.operating, "bind-10").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("intent rows=%d", n)
	}
}

func TestPOD_BIND_11(t *testing.T) {
	fix := seedAction(t, true)
	other := extraAction(t, fix, "DELIVERY", 2)
	if _, err := env.svc.CreateUploadIntent(env.ctx, fix.input("bind-11")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.CreateUploadIntent(env.ctx, other.input("bind-11")); codeOf(err) != apperrors.CodeConflict {
		t.Fatalf("other action: %v", err)
	}
}

func TestPOD_BIND_11B(t *testing.T) {
	fix := seedAction(t, true)
	in := fix.input("")
	before := intentCount(t, fix.operating)
	if _, err := env.svc.CreateUploadIntent(env.ctx, in); codeOf(err) != apperrors.CodeValidation {
		t.Fatalf("missing key: %v", err)
	}
	if intentCount(t, fix.operating) != before {
		t.Fatal("missing key inserted an intent")
	}
}

func TestPOD_BIND_12(t *testing.T) {
	fix := seedAction(t, true)
	done := finish(t, fix.input("bind-12"))
	again, err := env.svc.CompleteUpload(env.ctx, done.UploadID, "abc", domain.SubjectConfirmation{})
	if err != nil {
		t.Fatal(err)
	}
	if again.EvidenceID != done.EvidenceID || countEvidence(t, done.DocumentID) != 1 {
		t.Fatalf("retry id=%s count=%d", again.EvidenceID, countEvidence(t, done.DocumentID))
	}
}

func TestCompleteBindingRollsBackTogether(t *testing.T) {
	fix := seedAction(t, true)
	in := fix.input("bind-atomic")
	intent, err := env.svc.CreateUploadIntent(env.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.svc.UploadContent(env.ctx, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("jpeg")), domain.SubjectConfirmation{}); err != nil {
		t.Fatal(err)
	}
	var versionID uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT document_version_id FROM documents.pod_delivery_action_upload_intent WHERE id=$1`, intent.UploadID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	fileID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO documents.document_files (
			id, document_id, document_version_id, file_type, storage_provider, object_key
		) VALUES ($1,$2,$3,'IMAGE','local','manual')`, fileID, intent.DocumentID, versionID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO documents.pod_delivery_action_evidence (
			id, upload_intent_id, document_tenant_id, operating_tenant_id, execution_id, execution_action_id,
			execution_stop_id, shipment_id, shipment_tenant_id, cargo_id, owner_company_id, author_driver_id,
			author_company_id, document_id, document_version_id, document_file_id, idempotency_key
		) VALUES ($1,$2,$3,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		uuid.New(), intent.UploadID, fix.operating, fix.execution, fix.action, fix.stop, fix.shipment, fix.shipmentTenant,
		fix.cargo, fix.company, fix.driver, fix.authorCompany, intent.DocumentID, versionID, fileID, "bind-atomic"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.CompleteUpload(env.ctx, intent.UploadID, "abc", domain.SubjectConfirmation{}); err == nil {
		t.Fatal("expected complete to fail on the existing binding")
	}
	var status string
	var files int
	if err := env.pool.QueryRow(env.ctx, `SELECT status FROM documents.pod_delivery_action_upload_intent WHERE id=$1`, intent.UploadID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM documents.document_files WHERE document_id=$1`, intent.DocumentID).Scan(&files); err != nil {
		t.Fatal(err)
	}
	if status != "uploaded" || files != 1 || countEvidence(t, intent.DocumentID) != 1 {
		t.Fatalf("partial commit status=%s files=%d evidence=%d", status, files, countEvidence(t, intent.DocumentID))
	}
}

func TestPOD_BIND_13And14(t *testing.T) {
	fix := seedAction(t, true)
	other := extraAction(t, fix, "DELIVERY", 3)
	first := finish(t, fix.input("bind-13-a"))
	second := finish(t, other.input("bind-13-b"))
	left, err := env.svc.GetEvidence(env.ctx, fix.operating, fix.execution, fix.action)
	if err != nil || len(left) != 1 || left[0].DocumentID != first.DocumentID {
		t.Fatalf("left %+v %v", left, err)
	}
	right, err := env.svc.GetEvidence(env.ctx, other.operating, other.execution, other.action)
	if err != nil || len(right) != 1 || right[0].DocumentID != second.DocumentID {
		t.Fatalf("right %+v %v", right, err)
	}
	successor := extraAction(t, fix, "DELIVERY", 4)
	rows, err := env.svc.GetEvidence(env.ctx, successor.operating, successor.execution, successor.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("successor %+v %v", rows, err)
	}
}

func TestPOD_BIND_15(t *testing.T) {
	fix := seedAction(t, false)
	intent, err := env.legacy.CreateUploadIntent(env.ctx, service.CreatePODUploadInput{
		TenantID: fix.operating, ShipmentID: fix.shipment, DriverID: fix.driver,
		OwnerCompanyID: fix.company, MimeType: "image/jpeg", FileName: "legacy.jpg", IdempotencyKey: "legacy-15",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.legacy.UploadContent(env.ctx, fix.operating, intent.UploadID, intent.UploadToken, bytes.NewReader([]byte("legacy"))); err != nil {
		t.Fatal(err)
	}
	if _, err := env.legacy.CompleteUpload(env.ctx, service.CompletePODUploadInput{TenantID: fix.operating, UploadID: intent.UploadID, DriverID: fix.driver}); err != nil {
		t.Fatal(err)
	}
	if countEvidence(t, intent.DocumentID) != 0 {
		t.Fatal("historical shipment POD was bound to an action")
	}
	rows, err := env.svc.GetEvidence(env.ctx, fix.operating, fix.execution, fix.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("historical document surfaced: %+v %v", rows, err)
	}
}

func TestPOD_BIND_16And18(t *testing.T) {
	fix := seedAction(t, true)
	if err := insertUnknownDecision(t, fix); err != nil {
		t.Fatal(err)
	}
	before := countDecisions(t, fix.action)
	rows, err := env.svc.GetEvidence(env.ctx, fix.operating, fix.execution, fix.action)
	if err != nil || len(rows) != 0 || countDecisions(t, fix.action) != before {
		t.Fatalf("missing evidence mutated requirement rows=%d err=%v decisions=%d", len(rows), err, countDecisions(t, fix.action))
	}
	done := finish(t, fix.input("bind-16"))
	if countDecisions(t, fix.action) != before {
		t.Fatal("evidence presence wrote a requirement")
	}
	got, err := env.svc.GetEvidence(env.ctx, fix.operating, fix.execution, fix.action)
	if err != nil || len(got) != 1 || got[0].DocumentID != done.DocumentID {
		t.Fatalf("present %+v %v", got, err)
	}
}

func TestPOD_BIND_17(t *testing.T) {
	fix := seedAction(t, true)
	done := finish(t, fix.input("bind-17"))
	if err := insertKnownNotRequired(t, fix); err != nil {
		t.Fatal(err)
	}
	rows, err := env.svc.GetEvidence(env.ctx, fix.operating, fix.execution, fix.action)
	if err != nil || len(rows) != 1 || rows[0].DocumentID != done.DocumentID || countEvidence(t, done.DocumentID) != 1 {
		t.Fatalf("not-required deleted evidence %+v %v", rows, err)
	}
	if countDecisions(t, fix.action) != 1 {
		t.Fatal("evidence read deleted the requirement decision")
	}
}

func TestPOD_BIND_19(t *testing.T) {
	pool, err := pgxpool.New(env.ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	repo := repository.NewPODActionEvidenceRepository(pool)
	rows, err := repo.ListPresentEvidence(env.ctx, uuid.New(), uuid.New(), uuid.New())
	if err == nil || rows != nil && len(rows) == 0 && err == nil {
		t.Fatalf("closed pool returned rows=%v err=%v", rows, err)
	}
}

func TestPOD_BIND_20(t *testing.T) {
	fix := seedAction(t, true)
	done := finish(t, fix.input("bind-20"))
	rows, err := env.svc.GetEvidence(env.ctx, fix.shipmentTenant, fix.execution, fix.action)
	if err != nil || len(rows) != 0 {
		t.Fatalf("cross-tenant read %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.DocumentID == done.DocumentID {
			t.Fatal("cross-tenant read leaked document id")
		}
	}
}

func TestPODEvidenceUpdateImmutable(t *testing.T) {
	fix := seedAction(t, true)
	done := finish(t, fix.input("bind-immutable"))
	_, err := env.pool.Exec(env.ctx, `UPDATE documents.pod_delivery_action_evidence SET shipment_id=$2 WHERE document_id=$1`, done.DocumentID, uuid.New())
	if err == nil || !strings.Contains(err.Error(), "POD_ACTION_EVIDENCE_IMMUTABLE") {
		t.Fatalf("evidence update err=%v", err)
	}
	var shipment uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT shipment_id FROM documents.pod_delivery_action_evidence WHERE document_id=$1`, done.DocumentID).Scan(&shipment); err != nil {
		t.Fatal(err)
	}
	if shipment != fix.shipment {
		t.Fatal("evidence shipment pin changed")
	}
}

func TestMigration000099UpDownUp(t *testing.T) {
	ctx := env.ctx
	dbName := "pod_mig_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	testURL, cleanup, err := createDatabase(ctx, adminURL, dbName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanup(ctx) })
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := applyAllUp(ctx, pool, env.migDir); err != nil {
		t.Fatal(err)
	}
	if !tableExists(t, pool, "documents.pod_delivery_action_evidence") {
		t.Fatal("up did not create evidence table")
	}
	if !chainHeadIs(t, 102) {
		t.Fatal("migration chain is not contiguous through 000102")
	}
	down, err := os.ReadFile(filepath.Join(env.migDir, "000099_edo_pod_delivery_action_evidence_binding_v0_1b.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	if tableExists(t, pool, "documents.pod_delivery_action_evidence") || tableExists(t, pool, "documents.pod_delivery_action_upload_intent") {
		t.Fatal("down left action tables")
	}
	if !tableExists(t, pool, "transport.transport_execution_pod_requirement_decisions") {
		t.Fatal("down touched migration 000098")
	}
	up, err := os.ReadFile(filepath.Join(env.migDir, "000099_edo_pod_delivery_action_evidence_binding_v0_1b.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	if !tableExists(t, pool, "documents.pod_delivery_action_evidence") {
		t.Fatal("second up did not recreate evidence")
	}
}

func insertUnknownDecision(t *testing.T, fix actionFixture) error {
	t.Helper()
	_, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_pod_requirement_decisions (
			id, decision_version, operating_tenant_id, execution_id, execution_action_id,
			execution_stop_id, shipment_id, cargo_id, decision_state, created_at
		) VALUES ($1,1,$2,$3,$4,$5,$6,$7,'UNKNOWN', now())`,
		uuid.New(), fix.operating, fix.execution, fix.action, fix.stop, fix.shipment, fix.cargo)
	return err
}

func insertKnownNotRequired(t *testing.T, fix actionFixture) error {
	t.Helper()
	revision := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_revisions (
			id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
			source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
			status, contract_sha256, created_at, updated_at
		) VALUES ($1,$2,$3,$4,1,$5,1,'pod-bind-17','DEPOT_START','ACTIVE',$6,now(),now())`,
		revision, fix.execution, fix.operating, uuid.New(), uuid.New(), strings.Repeat("ab", 32)); err != nil {
		return err
	}
	_, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_execution_pod_requirement_decisions (
			id, decision_version, operating_tenant_id, execution_id, execution_action_id,
			execution_stop_id, shipment_id, cargo_id, decision_state, pod_required,
			requirement_decided_at, execution_revision_id, source_policy_id, source_policy_version,
			decision_actor_class, created_at
		) VALUES ($1,1,$2,$3,$4,$5,$6,$7,'KNOWN',false,now(),$8,$9,1,'SYSTEM',now())`,
		uuid.New(), fix.operating, fix.execution, fix.action, fix.stop, fix.shipment, fix.cargo, revision, uuid.New())
	return err
}

func intentCount(t *testing.T, tenant uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(env.ctx, `SELECT COUNT(*) FROM documents.pod_delivery_action_upload_intent WHERE document_tenant_id=$1`, tenant).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func codeOf(err error) apperrors.Code {
	var app *apperrors.AppError
	if errors.As(err, &app) {
		return app.Code
	}
	return ""
}

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(env.ctx, `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func chainHeadIs(t *testing.T, head int) bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(env.migDir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return len(files) == head
}
