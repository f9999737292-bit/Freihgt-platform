//go:build integration

package edo03i2

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestI2AttachmentFoundation(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	if err := execMigration(ctx, pool, "000095_edo_0_3_i2_signed_file_attachment.up.sql"); err != nil {
		t.Fatalf("I2-15 up: %v", err)
	}
	root := t.TempDir()
	store, err := storage.NewLocalObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.NewDocumentRepository(pool)
	docs := service.NewDocumentService(repo)
	attachments := service.NewAttachmentService(repo, store, repository.NewAttachmentRepository(pool))
	router := chi.NewRouter()
	h := handlers.NewAttachmentHandler(attachments)
	router.Post("/v1/documents/{id}/attachments", h.Create)
	router.Get("/v1/documents/{id}/attachments/{attachmentId}", h.Get)
	router.Get("/v1/documents/{id}/attachments/{attachmentId}/content", h.Content)
	router.Post("/v1/documents/{id}/attachments/{attachmentId}/finalize", h.Finalize)
	router.Post("/v1/documents/{id}/attachments/{attachmentId}/signatures", h.AttachSignature)
	router.Get("/v1/documents/{id}/attachments/{attachmentId}/signatures/{signatureId}", h.GetSignature)

	tenant := uuid.New()
	other := uuid.New()
	doc := mustInsertDocument(t, pool, tenant, "I2-A", "DRAFT")
	foreignDoc := mustInsertDocument(t, pool, other, "I2-B", "DRAFT")
	body := []byte("%PDF-1.7\nact")

	rec := upload(router, doc, tenant, "act.pdf", "application/pdf", "idem-1", "", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("I2-01 create status=%d body=%s", rec.Code, rec.Body.String())
	}
	created := decode(t, rec)
	if created["sha256"] == "" || created["status"] != "DRAFT" {
		t.Fatalf("I2-02 missing server hash: %#v", created)
	}
	attachmentID := created["attachment_id"].(string)

	badHash := upload(router, doc, tenant, "act.pdf", "application/pdf", "", strings.Repeat("ab", 32), body)
	if badHash.Code != http.StatusBadRequest {
		t.Fatalf("I2-03 wrong hash status=%d", badHash.Code)
	}
	replay := upload(router, doc, tenant, "act.pdf", "application/pdf", "idem-1", "", body)
	replayBody := decode(t, replay)
	if replay.Code != http.StatusCreated || replayBody["attachment_id"] != attachmentID {
		t.Fatalf("I2-12 replay status=%d %#v", replay.Code, replayBody)
	}
	var createdEvents int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM documents.attachment_audit_events
		WHERE attachment_id = $1 AND event_type = 'edo.attachment.created'`, attachmentID).Scan(&createdEvents); err != nil {
		t.Fatal(err)
	}
	if createdEvents != 1 {
		t.Fatalf("I2-13 created events=%d", createdEvents)
	}

	cross := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID, other, nil, nil)
	if cross.Code != http.StatusNotFound {
		t.Fatalf("I2-04 metadata status=%d", cross.Code)
	}
	crossFile := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/content", other, nil, nil)
	if crossFile.Code != http.StatusNotFound {
		t.Fatalf("I2-04 download status=%d", crossFile.Code)
	}
	spoof := upload(router, foreignDoc, tenant, "act.pdf", "application/pdf", "", "", body)
	if spoof.Code != http.StatusNotFound {
		t.Fatalf("I2-04 foreign document status=%d", spoof.Code)
	}
	querySpoof := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"?tenant_id="+other.String(), tenant, nil, nil)
	if querySpoof.Code != http.StatusUnauthorized {
		t.Fatalf("I2-04 query spoof status=%d", querySpoof.Code)
	}

	finalized := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/finalize", tenant, nil, nil)
	if finalized.Code != http.StatusOK || decode(t, finalized)["status"] != "FINALIZED" {
		t.Fatalf("I2-05 finalize status=%d body=%s", finalized.Code, finalized.Body.String())
	}
	again := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/finalize", tenant, nil, nil)
	if again.Code != http.StatusOK {
		t.Fatalf("I2-12 second finalize status=%d", again.Code)
	}
	var finalizeEvents int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM documents.attachment_audit_events
		WHERE attachment_id = $1 AND event_type = 'edo.attachment.finalized'`, attachmentID).Scan(&finalizeEvents)
	if finalizeEvents != 1 {
		t.Fatalf("I2-13 finalize events=%d", finalizeEvents)
	}
	_, err = pool.Exec(ctx, `UPDATE documents.document_attachments SET sha256 = $2 WHERE id = $1`, attachmentID, strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("I2-06 finalized hash update was allowed")
	}

	sigBody := []byte(`{"signature_format":"CAdES","signature_reference":"objects/sig-1","certificate_subject":"CN=Test","certificate_issuer":"CN=CA","certificate_serial":"01","certificate_thumbprint":"` + strings.Repeat("ab", 32) + `"}`)
	sig := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Idempotency-Key": "sig-1"}, sigBody)
	if sig.Code != http.StatusCreated {
		t.Fatalf("I2-07 signature status=%d body=%s", sig.Code, sig.Body.String())
	}
	sigJSON := decode(t, sig)
	if sigJSON["verification_status"] != "UNVERIFIED" {
		t.Fatalf("I2-08 status=%v", sigJSON["verification_status"])
	}
	sigReplay := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, map[string]string{"Idempotency-Key": "sig-1"}, sigBody)
	if decode(t, sigReplay)["signature_id"] != sigJSON["signature_id"] {
		t.Fatal("I2-12 signature replay created a new row")
	}
	var sigEvents int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM documents.attachment_audit_events
		WHERE attachment_id = $1 AND event_type = 'edo.signature.attached'`, attachmentID).Scan(&sigEvents)
	if sigEvents != 1 {
		t.Fatalf("I2-13 signature events=%d", sigEvents)
	}
	var verifiedEvents int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM documents.attachment_audit_events
		WHERE event_type IN ('edo.signature.verified', 'edo.signature.verification_failed')`).Scan(&verifiedEvents)
	if verifiedEvents != 0 {
		t.Fatalf("I2-08 verified events=%d", verifiedEvents)
	}
	crossSig := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures/"+sigJSON["signature_id"].(string), other, nil, nil)
	if crossSig.Code != http.StatusNotFound {
		t.Fatalf("I2-04 signature read status=%d", crossSig.Code)
	}
	_, err = pool.Exec(ctx, `UPDATE documents.attachment_signatures SET verification_status = 'VALID' WHERE id = $1`, sigJSON["signature_id"])
	if err == nil {
		t.Fatal("I2-08 VALID status was stored")
	}

	var storageKey string
	if err := pool.QueryRow(ctx, `SELECT storage_key FROM documents.document_attachments WHERE id = $1`, attachmentID).Scan(&storageKey); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(storageKey)), []byte("%PDF-tampered"), 0o640); err != nil {
		t.Fatal(err)
	}
	tamper := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/content", tenant, nil, nil)
	if tamper.Code != http.StatusConflict {
		t.Fatalf("I2-09 tamper status=%d", tamper.Code)
	}

	traversal := upload(router, doc, tenant, "../secret.pdf", "application/pdf", "", "", body)
	if traversal.Code != http.StatusBadRequest {
		t.Fatalf("I2-10 traversal status=%d", traversal.Code)
	}
	huge := bytes.Repeat([]byte("a"), domain.MaxAttachmentBytes+1)
	oversize := upload(router, doc, tenant, "big.bin", "application/octet-stream", "", "", huge)
	if oversize.Code != http.StatusBadRequest {
		t.Fatalf("I2-11 oversize status=%d", oversize.Code)
	}
	empty := upload(router, doc, tenant, "empty.pdf", "application/pdf", "", "", nil)
	if empty.Code != http.StatusBadRequest {
		t.Fatalf("zero-byte status=%d", empty.Code)
	}
	spoofType := upload(router, doc, tenant, "fake.pdf", "application/pdf", "", "", []byte("not-a-pdf"))
	if spoofType.Code != http.StatusBadRequest {
		t.Fatalf("content-type spoof status=%d", spoofType.Code)
	}
	privateKey := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, nil, []byte(`{"signature_format":"PKCS7","signature_reference":"-----BEGIN PRIVATE KEY-----\nMII"}`))
	if privateKey.Code != http.StatusBadRequest {
		t.Fatalf("private key status=%d", privateKey.Code)
	}

	signed := mustInsertDocument(t, pool, tenant, "I2-SIGNED", "SIGNED")
	_, err = docs.AddFile(ctx, signed, domain.CreateDocumentFileInput{
		TenantID: tenant, DocumentVersionID: uuid.New(), FileType: "PDF", ObjectKey: "legacy-key",
	})
	if err == nil {
		t.Fatal("I2-18 signed AddFile was allowed")
	}
}

func TestI2MigrationRoundTrip(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	if err := execMigration(ctx, pool, "000095_edo_0_3_i2_signed_file_attachment.up.sql"); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := execMigration(ctx, pool, "000095_edo_0_3_i2_signed_file_attachment.down.sql"); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := execMigration(ctx, pool, "000095_edo_0_3_i2_signed_file_attachment.up.sql"); err != nil {
		t.Fatalf("up again: %v", err)
	}
	tenant := uuid.New()
	doc := mustInsertDocument(t, pool, tenant, "I2-DOWN", "DRAFT")
	if _, err := pool.Exec(ctx, `
		INSERT INTO documents.document_attachments (
			document_id, tenant_id, file_name, media_type, size_bytes, sha256, storage_key
		) VALUES ($1,$2,'a.pdf','application/pdf',1,$3,'tenants/x')`,
		doc, tenant, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if err := execMigration(ctx, pool, "000095_edo_0_3_i2_signed_file_attachment.down.sql"); err == nil {
		t.Fatal("nonempty down was allowed")
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents.document_attachments`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("data preserved n=%d err=%v", n, err)
	}
}

func TestI2OpenAPIParity(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "packages", "openapi", "document-service.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(raw)
	for _, path := range []string{
		"/api/v1/documents/{id}/attachments:",
		"/api/v1/documents/{id}/attachments/{attachmentId}:",
		"/api/v1/documents/{id}/attachments/{attachmentId}/content:",
		"/api/v1/documents/{id}/attachments/{attachmentId}/finalize:",
		"/api/v1/documents/{id}/attachments/{attachmentId}/signatures:",
		"/api/v1/documents/{id}/attachments/{attachmentId}/signatures/{signatureId}:",
		"operationId: post_document_attachment",
		"operationId: get_document_attachment_signature",
	} {
		if !strings.Contains(spec, path) {
			t.Fatalf("openapi missing %s", path)
		}
	}
}

func upload(router http.Handler, doc, tenant uuid.UUID, name, media, idem, hash string, body []byte) *httptest.ResponseRecorder {
	headers := map[string]string{"X-File-Name": name, "Content-Type": media}
	if idem != "" {
		headers["Idempotency-Key"] = idem
	}
	if hash != "" {
		headers["X-Content-SHA256"] = hash
	}
	return do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments", tenant, headers, body)
}

func do(router http.Handler, method, path string, tenant uuid.UUID, headers map[string]string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenant.String())
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode status=%d body=%s err=%v", rec.Code, rec.Body.String(), err)
	}
	return out
}
