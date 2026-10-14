package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const stagingCORSDefault = "http://localhost:3000,http://localhost:3001,http://localhost:5173,http://localhost:13000,http://127.0.0.1:13000,http://localhost:3002"

func stagingCORSOrigins() []string {
	return strings.Split(stagingCORSDefault, ",")
}

func TestCORS_AllowsBintransPilotWebAdminOrigin(t *testing.T) {
	allowed := []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"http://localhost:5173",
		"http://localhost:13000",
		"http://127.0.0.1:13000",
	}
	handler := CORS(allowed)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:13000")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:13000" {
		t.Fatalf("Access-Control-Allow-Origin=%q want http://localhost:13000", got)
	}
	exposed := exposedHeaderNames(rec.Header())
	for _, name := range []string{"x-request-id", "content-disposition", "x-content-sha256"} {
		if _, ok := exposed[name]; !ok {
			t.Fatalf("Access-Control-Expose-Headers=%q missing %s", rec.Header().Get("Access-Control-Expose-Headers"), name)
		}
	}
}

func TestCORS_OptionsPreflightForPilotWebAdminOrigin(t *testing.T) {
	allowed := []string{
		"http://localhost:3000",
		"http://localhost:13000",
	}
	handler := CORS(allowed)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next handler must not run for OPTIONS preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:13000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:13000" {
		t.Fatalf("Access-Control-Allow-Origin=%q want http://localhost:13000", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" || !strings.Contains(got, "X-User-ID") {
		t.Fatalf("Access-Control-Allow-Headers=%q must include X-User-ID", got)
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key") {
		t.Fatalf("Access-Control-Allow-Headers=%q must include Idempotency-Key", rec.Header().Get("Access-Control-Allow-Headers"))
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Accept") {
		t.Fatalf("Access-Control-Allow-Headers=%q must include Accept", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCORS_RejectsUnknownOrigin(t *testing.T) {
	handler := CORS([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin=%q want empty", got)
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "" {
		t.Fatalf("Access-Control-Expose-Headers=%q want empty", got)
	}
}

func TestCORS_OptionsPreflightAllowsCarrierPortalOrigin(t *testing.T) {
	handler := CORS(stagingCORSOrigins())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next handler must not run for OPTIONS preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/carrier/documents", nil)
	req.Header.Set("Origin", "http://localhost:3002")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3002" {
		t.Fatalf("Access-Control-Allow-Origin=%q want http://localhost:3002", got)
	}
}

func TestCORS_OptionsPreflightOmitsUnlistedOrigin(t *testing.T) {
	handler := CORS(stagingCORSOrigins())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next handler must not run for OPTIONS preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/carrier/documents", nil)
	req.Header.Set("Origin", "http://localhost:3999")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin=%q want empty", got)
	}
}

func TestStagingComposeCORSAllowsCarrierPortalOrigin(t *testing.T) {
	body := string(readStagingCompose(t))
	const marker = "CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS:-"
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatal("staging compose CORS default not found")
	}
	rest := body[start+len(marker):]
	end := strings.Index(rest, "}")
	if end < 0 {
		t.Fatal("staging compose CORS default is unterminated")
	}
	list := rest[:end]
	if list != stagingCORSDefault {
		t.Fatalf("staging CORS default=%q", list)
	}
	if strings.Contains(list, "*") {
		t.Fatal("staging compose must not use a wildcard origin")
	}
	if strings.Contains(list, "http://127.0.0.1:3002") {
		t.Fatal("staging compose must not add 127.0.0.1:3002")
	}
}

func readStagingCompose(t *testing.T) []byte {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "infrastructure", "docker-compose", "docker-compose.bintrans-ct-staging.yml")
		body, err := os.ReadFile(candidate)
		if err == nil {
			return body
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("staging compose file not found")
		}
		dir = parent
	}
}

func exposedHeaderNames(header http.Header) map[string]struct{} {
	names := map[string]struct{}{}
	for _, part := range strings.Split(header.Get("Access-Control-Expose-Headers"), ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		names[name] = struct{}{}
	}
	return names
}
