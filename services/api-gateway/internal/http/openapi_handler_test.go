package http_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestOpenAPIEndpoints(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "openapi.yaml", "openapi: 3.0.3\ninfo:\n  title: Test\n")
	writeFile(t, dir, "openapi.json", `{"openapi":"3.0.3"}`)
	writeFile(t, dir, "identity-service.yaml", "openapi: 3.0.3\n")

	openAPI := gatewayhttp.NewOpenAPIHandler(dir)
	r := chi.NewRouter()
	openAPI.RegisterRoutes(r)

	t.Run("docs returns HTML", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("content-type=%q", ct)
		}
		if body := rec.Body.String(); !strings.Contains(body, "swagger-ui") {
			t.Fatalf("expected swagger-ui in body")
		}
	})

	t.Run("openapi yaml", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/yaml" {
			t.Fatalf("content-type=%q", ct)
		}
	})

	t.Run("openapi json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type=%q", ct)
		}
	})

	t.Run("openapi index", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, `"documents"`) {
			t.Fatalf("body=%s", body)
		}
	})

	t.Run("service yaml file", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi/identity-service.yaml", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/openapi/..%2F.env", nil)
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatal("expected path traversal to be blocked")
		}
	})

	t.Run("invalid extension blocked", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi/.env", nil))
		if rec.Code == http.StatusOK {
			t.Fatal("expected invalid extension to be blocked")
		}
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
