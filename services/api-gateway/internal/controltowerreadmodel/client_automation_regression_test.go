package controltowerreadmodel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyAutomationJSONUsesReadModelBaseURL(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.String()
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client(), Config{BaseURL: srv.URL, MaxResponseBytes: 1 << 20}, NewMetrics())
	_, depErr := client.ProxyAutomationJSON(context.Background(), http.MethodGet, "tenant", "user", "req-1", AutomationEvaluatePath, nil)
	if depErr != nil {
		t.Fatalf("ProxyAutomationJSON: %v", depErr.Err)
	}
	want := AutomationEvaluatePath
	if seenPath != want {
		t.Fatalf("request path = %q, want %q", seenPath, want)
	}
	if !strings.Contains(srv.URL, "127.0.0.1") {
		t.Fatalf("expected local test server URL")
	}
}

func TestProxyAutomationJSONRelativePathCannotHitWrongHost(t *testing.T) {
	var host string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host = r.Host
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.Client(), Config{BaseURL: srv.URL, MaxResponseBytes: 1 << 20}, NewMetrics())
	path := AutomationRulesPath + "?status=active"
	_, depErr := client.ProxyAutomationJSON(context.Background(), http.MethodGet, "tenant", "user", "req-2", path, nil)
	if depErr != nil {
		t.Fatalf("ProxyAutomationJSON: %v", depErr.Err)
	}
	if host != srv.Listener.Addr().String() {
		t.Fatalf("host = %q, want read-model server %q", host, srv.Listener.Addr().String())
	}
}
