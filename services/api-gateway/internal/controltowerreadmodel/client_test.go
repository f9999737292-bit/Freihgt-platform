package controltowerreadmodel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, serverURL string, timeout time.Duration) *Client {
	t.Helper()
	return NewClient(&http.Client{Timeout: timeout}, Config{
		BaseURL:          serverURL,
		Timeout:          timeout,
		MaxResponseBytes: 256 * 1024,
	}, NewMetrics())
}

func validSummaryJSON() string {
	return `{
		"totalShipments": 2,
		"byStatus": {"IN_TRANSIT": 1, "DELIVERED": 1},
		"incompleteProjections": 0,
		"freshness": {"consumerRunning": true}
	}`
}

func TestClientUsesFixedPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validSummaryJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, err := client.FetchStatusSummary(context.Background(), ModeShadow, "11111111-1111-1111-1111-111111111111", "req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/internal/v1/control-tower/status-summary" {
		t.Fatalf("path=%q want fixed internal path", gotPath)
	}
}

func TestClientSendsVerifiedTenantAndRequestID(t *testing.T) {
	var gotTenant, gotRequestID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotRequestID = r.Header.Get("X-Request-ID")
		_, _ = w.Write([]byte(validSummaryJSON()))
	}))
	defer server.Close()

	tenant := "11111111-1111-1111-1111-111111111111"
	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, tenant, "req-trusted")
	if depErr != nil {
		t.Fatalf("unexpected dep error: %v", depErr)
	}
	if gotTenant != tenant {
		t.Fatalf("tenant=%q want %q", gotTenant, tenant)
	}
	if gotRequestID != "req-trusted" {
		t.Fatalf("request id=%q want req-trusted", gotRequestID)
	}
}

func TestClientDoesNotForwardAuthorizationOrIdentityHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, header := range []string{"Authorization", "Cookie", "X-User-Roles", "X-User-Email"} {
			if r.Header.Get(header) != "" {
				t.Errorf("unexpected header %s=%q", header, r.Header.Get(header))
			}
		}
		_, _ = w.Write([]byte(validSummaryJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModeShadow, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr != nil {
		t.Fatalf("unexpected dep error: %v", depErr)
	}
}

func TestClientDisabledModeSkipsRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	payload, depErr := client.FetchStatusSummary(context.Background(), ModeDisabled, "11111111-1111-1111-1111-111111111111", "req-1")
	if payload != nil || depErr != nil {
		t.Fatalf("expected nil payload and error, got payload=%v err=%v", payload, depErr)
	}
	if called {
		t.Fatal("read-model should not be called in disabled mode")
	}
}

func TestClientTimeoutClassified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(validSummaryJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, 50*time.Millisecond)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if depErr.Reason != ReasonTimeout {
		t.Fatalf("reason=%q want %q", depErr.Reason, ReasonTimeout)
	}
}

func TestClientNetworkErrorClassified(t *testing.T) {
	client := testClient(t, "http://127.0.0.1:1", 200*time.Millisecond)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if depErr.Reason != ReasonNetworkError {
		t.Fatalf("reason=%q want %q", depErr.Reason, ReasonNetworkError)
	}
}

func TestClientNon2XXClassified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if depErr.Reason != ReasonNon2XX {
		t.Fatalf("reason=%q want %q", depErr.Reason, ReasonNon2XX)
	}
}

func TestClientOversizedResponseRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 300*1024)))
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: time.Second}, Config{
		BaseURL:          server.URL,
		Timeout:          time.Second,
		MaxResponseBytes: 1024,
	}, NewMetrics())
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if depErr.Reason != ReasonMalformedResponse {
		t.Fatalf("reason=%q want %q", depErr.Reason, ReasonMalformedResponse)
	}
}

func TestClientInvalidJSONRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonMalformedResponse {
		t.Fatalf("expected malformed response, got %v", depErr)
	}
}

func TestClientInvalidStatusKeyRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"UNKNOWN_STATUS":1},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonInvalidContract {
		t.Fatalf("expected invalid contract, got %v", depErr)
	}
}

func TestClientNegativeCountRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"IN_TRANSIT":-1},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonInvalidContract {
		t.Fatalf("expected invalid contract, got %v", depErr)
	}
}

func TestClientIncompleteExceedsTotalRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{},"incompleteProjections":2,"freshness":{"consumerRunning":true}}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonInvalidContract {
		t.Fatalf("expected invalid contract, got %v", depErr)
	}
}

func TestClientConsumerRunningDecoded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":0,"byStatus":{},"incompleteProjections":0,"freshness":{"consumerRunning":false}}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	payload, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr != nil {
		t.Fatalf("unexpected error: %v", depErr)
	}
	if payload.Freshness.ConsumerRunning {
		t.Fatal("expected consumerRunning=false")
	}
}

func TestClientContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(validSummaryJSON()))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(ctx, ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if depErr.Reason != ReasonCancelled && depErr.Reason != ReasonNetworkError {
		t.Fatalf("reason=%q want cancelled or network", depErr.Reason)
	}
}

func TestDependencyErrorDoesNotExposeRawBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"secret":"internal-only","totalShipments":-1}`)
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if strings.Contains(depErr.Error(), "internal-only") {
		t.Fatalf("raw body leaked into error: %q", depErr.Error())
	}
}

func TestClientValidResponseDecoded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validSummaryJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	payload, depErr := client.FetchStatusSummary(context.Background(), ModePrimary, "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr != nil {
		t.Fatalf("unexpected error: %v", depErr)
	}
	if payload.TotalShipments != 2 {
		t.Fatalf("total=%d want 2", payload.TotalShipments)
	}
	if payload.ByStatus["IN_TRANSIT"] != 1 {
		t.Fatalf("byStatus=%v", payload.ByStatus)
	}
	var roundTrip map[string]any
	if err := json.Unmarshal([]byte(validSummaryJSON()), &roundTrip); err != nil {
		t.Fatalf("fixture json: %v", err)
	}
}
