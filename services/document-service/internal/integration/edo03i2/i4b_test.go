//go:build integration

package edo03i2

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/http/handlers"
	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

func TestI4BMigrationAndDetachedSignature(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	if err := execMigration(ctx, pool, "000095_edo_0_3_i2_signed_file_attachment.up.sql"); err != nil {
		t.Fatalf("095 up: %v", err)
	}
	if err := execMigration(ctx, pool, "000096_edo_0_3_i4b_signature_verification_foundation.up.sql"); err != nil {
		t.Fatalf("096 up: %v", err)
	}

	tenant := uuid.New()
	doc := mustInsertDocument(t, pool, tenant, "I4B-A", "DRAFT")
	store, err := storage.NewLocalObjectStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	attachments := service.NewAttachmentService(repository.NewDocumentRepository(pool), store, repository.NewAttachmentRepository(pool))
	router := chi.NewRouter()
	h := handlers.NewAttachmentHandler(attachments)
	router.Post("/v1/documents/{id}/attachments", h.Create)
	router.Post("/v1/documents/{id}/attachments/{attachmentId}/finalize", h.Finalize)
	router.Post("/v1/documents/{id}/attachments/{attachmentId}/signatures", h.AttachSignature)
	router.Get("/v1/documents/{id}/attachments/{attachmentId}/signatures/{signatureId}", h.GetSignature)

	created := decode(t, upload(router, doc, tenant, "act.pdf", "application/pdf", "", "", []byte("%PDF-1.7\nact")))
	attachmentID := created["attachment_id"].(string)
	if do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/finalize", tenant, nil, nil).Code != http.StatusOK {
		t.Fatal("finalize failed")
	}
	draftDoc := mustInsertDocument(t, pool, tenant, "I4B-DRAFT", "DRAFT")
	draft := decode(t, upload(router, draftDoc, tenant, "draft.pdf", "application/pdf", "", "", []byte("%PDF-1.7\ndraft")))
	draftSig := do(router, http.MethodPost, "/v1/documents/"+draftDoc.String()+"/attachments/"+draft["attachment_id"].(string)+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": "draft-1"}, []byte("draft-sig"))
	if draftSig.Code != http.StatusConflict {
		t.Fatalf("draft signature status=%d", draftSig.Code)
	}

	legacy := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/json"}, []byte(`{"signature_format":"CAdES","signature_reference":"objects/sig-1"}`))
	if legacy.Code != http.StatusCreated || decode(t, legacy)["verification_status"] != "UNVERIFIED" {
		t.Fatalf("legacy status=%d body=%s", legacy.Code, legacy.Body.String())
	}
	var legacyStatus string
	if err := pool.QueryRow(ctx, `SELECT verification_status FROM documents.attachment_signatures WHERE signature_reference = 'objects/sig-1'`).Scan(&legacyStatus); err != nil || legacyStatus != "UNVERIFIED" {
		t.Fatalf("legacy persisted %s err=%v", legacyStatus, err)
	}
	if err := execMigration(ctx, pool, "000096_edo_0_3_i4b_signature_verification_foundation.down.sql"); err != nil {
		t.Fatalf("096 down: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT verification_status FROM documents.attachment_signatures WHERE signature_reference = 'objects/sig-1'`).Scan(&legacyStatus); err != nil || legacyStatus != "UNVERIFIED" {
		t.Fatalf("legacy row after down %s err=%v", legacyStatus, err)
	}
	if err := execMigration(ctx, pool, "000096_edo_0_3_i4b_signature_verification_foundation.up.sql"); err != nil {
		t.Fatalf("096 up again: %v", err)
	}
	var backfill int
	if err := pool.QueryRow(ctx, `SELECT verification_status FROM documents.attachment_signatures WHERE signature_reference = 'objects/sig-1'`).Scan(&legacyStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.signature_verification_evidence`).Scan(&backfill); err != nil || backfill != 0 || legacyStatus != "UNVERIFIED" {
		t.Fatalf("backfill=%d status=%s err=%v", backfill, legacyStatus, err)
	}

	body := []byte("detached-signature-bytes")
	other := uuid.New()
	cross := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", other, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": "cross-1"}, body)
	if cross.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status=%d", cross.Code)
	}
	missingKey := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature"}, body)
	blankKey := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": "   "}, body)
	longKey := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": strings.Repeat("k", 129)}, body)
	if missingKey.Code != http.StatusBadRequest || blankKey.Code != http.StatusBadRequest || longKey.Code != http.StatusBadRequest {
		t.Fatalf("idempotency status missing=%d blank=%d long=%d", missingKey.Code, blankKey.Code, longKey.Code)
	}
	oversize := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": "oversize"}, bytesRepeat(domain.MaxSignatureBytes+1))
	if oversize.Code != http.StatusBadRequest {
		t.Fatalf("oversize status=%d", oversize.Code)
	}
	first := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": "bin-1"}, body)
	if first.Code != http.StatusCreated {
		t.Fatalf("binary status=%d body=%s", first.Code, first.Body.String())
	}
	got := decode(t, first)
	if got["verification_status"] != "PENDING" || got["verification_reason_code"] != "VERIFIER_UNAVAILABLE" {
		t.Fatalf("effective %#v", got)
	}
	if strings.Contains(got["signature_reference"].(string), "tenants/") {
		t.Fatal("storage key was returned")
	}
	var persisted, objectKey, sum string
	var size int64
	if err := pool.QueryRow(ctx, `
		SELECT s.verification_status, b.object_key, b.sha256, b.size_bytes
		FROM documents.attachment_signatures s
		JOIN documents.attachment_signature_blobs b ON b.signature_id = s.id
		WHERE s.id = $1`, got["signature_id"]).Scan(&persisted, &objectKey, &sum, &size); err != nil {
		t.Fatal(err)
	}
	if persisted != "UNVERIFIED" || size != int64(len(body)) || sum != domain.SHA256Hex(body) || strings.Contains(got["signature_reference"].(string), objectKey) {
		t.Fatalf("persisted=%s size=%d sum=%s key=%s", persisted, size, sum, objectKey)
	}
	replay := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": "bin-1"}, body)
	if decode(t, replay)["signature_id"] != got["signature_id"] {
		t.Fatal("replay created another signature")
	}
	var blobs, evidence, failed int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.attachment_signature_blobs`).Scan(&blobs)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.signature_verification_evidence`).Scan(&evidence)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.attachment_audit_events WHERE event_type = 'edo.signature.verification_failed'`).Scan(&failed)
	if blobs != 1 || evidence != 1 || failed != 0 {
		t.Fatalf("blobs=%d evidence=%d failed=%d", blobs, evidence, failed)
	}
	conflict := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Content-Type": "application/pkcs7-signature", "Idempotency-Key": "bin-1"}, []byte("different"))
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d", conflict.Code)
	}
	var docStatus string
	var sessions, legacySignatures int
	if err := pool.QueryRow(ctx, `SELECT document_status FROM documents.documents WHERE id = $1`, doc).Scan(&docStatus); err != nil || docStatus != "DRAFT" {
		t.Fatalf("document status=%s err=%v", docStatus, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.signing_sessions`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("signing sessions=%d err=%v", sessions, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.signatures`).Scan(&legacySignatures); err != nil || legacySignatures != 0 {
		t.Fatalf("legacy signatures=%d err=%v", legacySignatures, err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.signature_verification_evidence (
			signature_id, tenant_id, verification_status, reason_code, attempted_at, verifier_version, policy_id, policy_version
		) VALUES ($1,$2,'VALID','VERIFIER_UNAVAILABLE', now(), 'x','QUALIFIED_CADES_BES','v1')`, got["signature_id"], tenant)
	if err == nil {
		t.Fatal("VALID evidence was stored")
	}
	_, err = pool.Exec(ctx, `UPDATE documents.signature_verification_evidence SET reason_code = 'VERIFIER_UNAVAILABLE'`)
	if err == nil {
		t.Fatal("evidence update was allowed")
	}
	_, err = pool.Exec(ctx, `DELETE FROM documents.attachment_signature_blobs`)
	if err == nil {
		t.Fatal("blob delete was allowed")
	}
	var legacyID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM documents.attachment_signatures WHERE signature_reference = 'objects/sig-1'`).Scan(&legacyID); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.signature_verification_evidence (
			signature_id, tenant_id, verification_status, reason_code, attempted_at, verifier_version, policy_id, policy_version
		) VALUES ($1,$2,'PENDING','VERIFIER_UNAVAILABLE', now(), 'x','QUALIFIED_CADES_BES','v1')`, legacyID, tenant)
	if err == nil {
		t.Fatal("evidence without a server blob was stored")
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO documents.signature_verification_evidence (
			signature_id, tenant_id, verification_status, reason_code, attempted_at, verifier_version, policy_id, policy_version
		) VALUES ($1,$2,'PENDING','VERIFIER_UNAVAILABLE', now(), 'x','OTHER_POLICY','v9')`, got["signature_id"], tenant)
	if err != nil {
		t.Fatalf("other-policy evidence: %v", err)
	}
	evidenceRepo := repository.NewAttachmentRepository(pool)
	binaryID := uuid.MustParse(got["signature_id"].(string))
	status, reason, err := evidenceRepo.LatestEvidence(ctx, tenant, binaryID, domain.PolicyQualifiedCAdES, domain.PolicyVersionV1)
	if err != nil || status != "PENDING" || reason != "VERIFIER_UNAVAILABLE" {
		t.Fatalf("qualified evidence status=%s reason=%s err=%v", status, reason, err)
	}
	otherOnly := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO documents.attachment_signatures (
			id, attachment_id, tenant_id, signature_format, signature_reference, verification_status
		) VALUES ($1,$2,$3,'CAdES',$4,'UNVERIFIED')`,
		otherOnly, attachmentID, tenant, "bintrans:attachment-signature:"+otherOnly.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO documents.signature_verification_history (signature_id, tenant_id, verification_status)
		VALUES ($1,$2,'UNVERIFIED')`, otherOnly, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO documents.attachment_signature_blobs (
			signature_id, tenant_id, object_key, size_bytes, sha256, media_type, profile
		) VALUES ($1,$2,$3,1,$4,'application/pkcs7-signature','CAdES-BES')`,
		otherOnly, tenant, "tenants/"+tenant.String()+"/documents/"+doc.String()+"/attachment-signatures/"+attachmentID+"/"+otherOnly.String(), strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO documents.signature_verification_evidence (
			signature_id, tenant_id, verification_status, reason_code, attempted_at, verifier_version, policy_id, policy_version
		) VALUES ($1,$2,'PENDING','VERIFIER_UNAVAILABLE', now(), 'x','OTHER_POLICY','v9')`, otherOnly, tenant); err != nil {
		t.Fatal(err)
	}
	otherStatus, otherReason, err := evidenceRepo.LatestEvidence(ctx, tenant, otherOnly, domain.PolicyQualifiedCAdES, domain.PolicyVersionV1)
	if err != nil || otherStatus != "" || otherReason != "" {
		t.Fatalf("other policy overrode qualified lookup status=%s reason=%s err=%v", otherStatus, otherReason, err)
	}
	foreignStatus, foreignReason, err := evidenceRepo.LatestEvidence(ctx, tenant, otherOnly, "OTHER_POLICY", "v9")
	if err != nil || foreignStatus != "PENDING" || foreignReason != "VERIFIER_UNAVAILABLE" {
		t.Fatalf("other policy lookup status=%s reason=%s err=%v", foreignStatus, foreignReason, err)
	}
	otherRead := decode(t, do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures/"+otherOnly.String(), tenant, nil, nil))
	if otherRead["verification_status"] != "UNVERIFIED" {
		t.Fatalf("other policy became effective %#v", otherRead)
	}
	read := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures/"+got["signature_id"].(string), tenant, nil, nil)
	readJSON := decode(t, read)
	if read.Code != http.StatusOK || readJSON["verification_status"] != "PENDING" || readJSON["verification_reason_code"] != "VERIFIER_UNAVAILABLE" {
		t.Fatalf("get %#v", readJSON)
	}
}

func bytesRepeat(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'a'
	}
	return out
}
