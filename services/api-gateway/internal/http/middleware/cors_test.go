package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
