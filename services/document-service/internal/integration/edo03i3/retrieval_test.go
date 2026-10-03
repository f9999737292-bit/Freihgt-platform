//go:build integration

package edo03i3

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/http/handlers"
	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

func TestI3SignedArtifactRetrieval(t *testing.T) {
	pool := newPool(t)
	stub := newMemS3()
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	store, err := storage.NewS3ObjectStore(storage.Config{
		Provider: "s3", Endpoint: server.URL, Region: "us-east-1", Bucket: "edo-attachments",
		AccessKey: "test-access", SecretKey: "test-secret", PathStyle: true, TLS: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.NewDocumentRepository(pool)
	attachments := service.NewAttachmentService(repo, store, repository.NewAttachmentRepository(pool))
	router := chi.NewRouter()
	h := handlers.NewAttachmentHandler(attachments)
	router.Post("/v1/documents/{id}/attachments", h.Create)
	router.Get("/v1/documents/{id}/attachments/{attachmentId}", h.Get)
	router.Get("/v1/documents/{id}/attachments/{attachmentId}/content", h.Content)
	router.Post("/v1/documents/{id}/attachments/{attachmentId}/finalize", h.Finalize)
	router.Post("/v1/documents/{id}/attachments/{attachmentId}/signatures", h.AttachSignature)

	tenant := uuid.New()
	other := uuid.New()
	doc := mustInsertDocument(t, pool, tenant, "I3-A", "DRAFT")
	otherDoc := mustInsertDocument(t, pool, tenant, "I3-C", "DRAFT")
	body := []byte("%PDF-1.7\nartifact")
	rec := upload(router, doc, tenant, "act.pdf", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	created := decode(t, rec)
	if _, ok := created["storage_key"]; ok {
		t.Fatal("raw storage key was exposed")
	}
	attachmentID := created["attachment_id"].(string)
	var storageKey, fileName string
	if err := pool.QueryRow(context.Background(), `SELECT storage_key, file_name FROM documents.document_attachments WHERE id = $1`, attachmentID).Scan(&storageKey, &fileName); err != nil {
		t.Fatal(err)
	}
	if fileName != "act.pdf" || strings.Contains(storageKey, "act.pdf") || strings.Contains(storageKey, "evil") || !strings.Contains(storageKey, "/attachments/") {
		t.Fatalf("key %s name %s", storageKey, fileName)
	}
	replay := upload(router, doc, tenant, "act.pdf", body)
	replay.Header().Set("Idempotency-Key", "same")
	// first upload had no idempotency key; a second distinct upload is a new object.
	second := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments", tenant, map[string]string{
		"X-File-Name": "act.pdf", "Content-Type": "application/pdf", "Idempotency-Key": "idem-i3", "X-Object-Key": "tenants/evil/raw",
	}, body)
	if second.Code != http.StatusCreated {
		t.Fatalf("idempotent create status=%d body=%s", second.Code, second.Body.String())
	}
	again := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments", tenant, map[string]string{
		"X-File-Name": "act.pdf", "Content-Type": "application/pdf", "Idempotency-Key": "idem-i3",
	}, body)
	if decode(t, again)["attachment_id"] != decode(t, second)["attachment_id"] {
		t.Fatal("retry created a second attachment")
	}

	if do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/content", other, nil, nil).Code != http.StatusNotFound {
		t.Fatal("cross-tenant download was allowed")
	}
	if do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/download-url", other, nil, nil).Code != http.StatusNotFound {
		t.Fatal("cross-tenant presign was not denied")
	}
	if do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+uuid.NewString(), tenant, nil, nil).Code != http.StatusNotFound {
		t.Fatal("guessed attachment id was found")
	}
	if do(router, http.MethodGet, "/v1/documents/"+otherDoc.String()+"/attachments/"+attachmentID, tenant, nil, nil).Code != http.StatusNotFound {
		t.Fatal("cross-document read was allowed")
	}
	if do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"?tenant_id="+other.String(), tenant, nil, nil).Code != http.StatusUnauthorized {
		t.Fatal("tenant spoof was accepted")
	}

	finalized := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/finalize", tenant, nil, nil)
	if finalized.Code != http.StatusOK {
		t.Fatalf("finalize status=%d body=%s", finalized.Code, finalized.Body.String())
	}
	if _, err := store.Put(context.Background(), storageKey, bytes.NewReader(body), int64(len(body))); err == nil {
		t.Fatal("finalized object overwrite was allowed")
	}
	parsedAttachment := uuid.MustParse(attachmentID)
	if err := attachments.DeleteDraft(context.Background(), tenant, doc, parsedAttachment); err == nil {
		t.Fatal("finalized delete was allowed")
	}
	content := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/content", tenant, nil, nil)
	if content.Code != http.StatusOK || !bytes.Equal(content.Body.Bytes(), body) || content.Header().Get("X-Content-SHA256") != domain.SHA256Hex(body) {
		t.Fatalf("retrieval status=%d sha=%s", content.Code, content.Header().Get("X-Content-SHA256"))
	}
	stub.corrupt(storageKey)
	if do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/content", tenant, nil, nil).Code != http.StatusConflict {
		t.Fatal("tampered object was served")
	}
	stub.restore(storageKey, body)
	sig := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/signatures", tenant, nil, []byte(`{"signature_format":"CAdES","signature_reference":"objects/sig-1"}`))
	if sig.Code != http.StatusCreated || decode(t, sig)["verification_status"] != "UNVERIFIED" {
		t.Fatalf("signature status=%d body=%s", sig.Code, sig.Body.String())
	}
	exact := do(router, http.MethodGet, "/v1/documents/"+doc.String()+"/attachments/"+attachmentID+"/content", tenant, nil, nil)
	if !bytes.Equal(exact.Body.Bytes(), body) {
		t.Fatal("signed artifact bytes changed")
	}
	traversal := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments", tenant, map[string]string{
		"X-File-Name": "../secret.pdf", "Content-Type": "application/pdf",
	}, body)
	if traversal.Code != http.StatusBadRequest {
		t.Fatalf("traversal status=%d", traversal.Code)
	}
	oversize := do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments", tenant, map[string]string{
		"X-File-Name": "big.bin", "Content-Type": "application/octet-stream",
	}, bytes.Repeat([]byte("a"), domain.MaxAttachmentBytes+1))
	if oversize.Code != http.StatusBadRequest {
		t.Fatalf("oversize status=%d", oversize.Code)
	}
	_ = replay
}

func upload(router http.Handler, doc, tenant uuid.UUID, name string, body []byte) *httptest.ResponseRecorder {
	return do(router, http.MethodPost, "/v1/documents/"+doc.String()+"/attachments", tenant, map[string]string{
		"X-File-Name": name, "Content-Type": "application/pdf",
	}, body)
}

func do(router http.Handler, method, path string, tenant uuid.UUID, headers map[string]string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", tenant.String())
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode status=%d body=%s err=%v", rec.Code, rec.Body.String(), err)
	}
	return out
}

type memS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemS3() *memS3 { return &memS3{objects: map[string][]byte{}} }

func (s *memS3) corrupt(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects["edo-attachments/"+strings.TrimPrefix(key, "/")] = []byte("%PDF-tampered")
	// path-style key stored without bucket prefix in the map below; keep both lookups aligned in ServeHTTP.
	s.objects[key] = []byte("%PDF-tampered")
}

func (s *memS3) restore(key string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = append([]byte(nil), body...)
}

func (s *memS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/edo-attachments/")
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		if _, ok := s.objects[key]; ok && r.Header.Get("If-None-Match") == "*" {
			http.Error(w, "exists", http.StatusPreconditionFailed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		s.objects[key] = body
		w.WriteHeader(http.StatusOK)
	case http.MethodGet, http.MethodHead:
		body, ok := s.objects[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", itoa(len(body)))
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(body)
	case http.MethodDelete:
		delete(s.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
