package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestS3PutGetRoundTripAndNoOverwrite(t *testing.T) {
	stub := newMemS3()
	server := httptest.NewServer(stub)
	defer server.Close()
	store := testS3(t, server.URL)
	ctx := context.Background()
	key := "tenants/11111111-1111-1111-1111-111111111111/documents/22222222-2222-2222-2222-222222222222/attachments/33333333-3333-3333-3333-333333333333"
	body := []byte("%PDF-1.7 roundtrip")
	n, err := store.Put(ctx, key, bytes.NewReader(body), int64(len(body)))
	if err != nil || n != int64(len(body)) {
		t.Fatalf("put n=%d err=%v", n, err)
	}
	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	read, _ := io.ReadAll(got)
	if !bytes.Equal(read, body) {
		t.Fatalf("roundtrip mismatch %q", read)
	}
	meta, err := store.Metadata(ctx, key)
	if err != nil || meta.SizeBytes != int64(len(body)) {
		t.Fatalf("metadata %+v err=%v", meta, err)
	}
	ok, err := store.Exists(ctx, key)
	if err != nil || !ok {
		t.Fatalf("exists %v %v", ok, err)
	}
	replacement := []byte("%PDF-1.7 replaced")
	if _, err := store.Put(ctx, key, bytes.NewReader(replacement), int64(len(replacement))); !errors.Is(err, ErrObjectExists) {
		t.Fatalf("overwrite err=%v", err)
	}
	got, err = store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	read, _ = io.ReadAll(got)
	if !bytes.Equal(read, body) {
		t.Fatalf("conditional put replaced original %q", read)
	}
	if _, err := store.Put(ctx, "../secret", bytes.NewReader(body), 100); err == nil {
		t.Fatal("traversal key was accepted")
	}
}

func TestS3ProviderErrorHidesSecret(t *testing.T) {
	secret := "super-secret-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider boom "+secret, http.StatusInternalServerError)
	}))
	defer server.Close()
	store, err := NewS3ObjectStore(Config{
		Provider: "s3", Endpoint: server.URL, Region: "us-east-1", Bucket: "edo-attachments",
		AccessKey: "test-access", SecretKey: secret, PathStyle: true, TLS: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Put(context.Background(), "tenants/a/documents/b/attachments/c", bytes.NewReader([]byte("x")), 10)
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("provider error leaked: %v", err)
	}
}

func TestLocalAdapterRoundTrip(t *testing.T) {
	store, err := NewLocalObjectStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := "tenants/a/documents/b/attachments/c"
	if _, err := store.Put(context.Background(), key, bytes.NewReader([]byte("pdf")), 10); err != nil {
		t.Fatal(err)
	}
	body, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, _ := io.ReadAll(body)
	if string(got) != "pdf" {
		t.Fatalf("local get %q", got)
	}
}

func testS3(t *testing.T, endpoint string) *S3ObjectStore {
	t.Helper()
	store, err := NewS3ObjectStore(Config{
		Provider: "s3", Endpoint: endpoint, Region: "us-east-1", Bucket: "edo-attachments",
		AccessKey: "test-access", SecretKey: "test-secret", PathStyle: true, TLS: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

type memS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemS3() *memS3 {
	return &memS3{objects: map[string][]byte{}}
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
	case http.MethodGet:
		body, ok := s.objects[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	case http.MethodHead:
		body, ok := s.objects[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		delete(s.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method", http.StatusMethodNotAllowed)
	}
}
