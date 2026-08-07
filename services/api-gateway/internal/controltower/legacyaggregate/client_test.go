package legacyaggregate

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

func validAggregateJSON() string {
	return `{
		"totalShipments": 2,
		"countedShipments": 2,
		"byStatus": {"IN_TRANSIT": 1, "DELIVERED": 1},
		"complete": true
	}`
}

func TestClientUsesFixedPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), "disabled", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr != nil {
		t.Fatalf("unexpected error: %v", depErr)
	}
	if gotPath != "/internal/v1/shipments/status-summary" {
		t.Fatalf("path=%q want fixed internal path", gotPath)
	}
}

func TestClientSendsVerifiedTenantAndRequestID(t *testing.T) {
	var gotTenant, gotRequestID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotRequestID = r.Header.Get("X-Request-ID")
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	defer server.Close()

	tenant := "11111111-1111-1111-1111-111111111111"
	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", tenant, "req-trusted")
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
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), "shadow", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr != nil {
		t.Fatalf("unexpected dep error: %v", depErr)
	}
}

func TestClientTimeoutClassified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, 50*time.Millisecond)
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if depErr.Reason != ReasonTimeout {
		t.Fatalf("reason=%q want %q", depErr.Reason, ReasonTimeout)
	}
}

func TestClientNetworkErrorClassified(t *testing.T) {
	client := testClient(t, "http://127.0.0.1:1", 200*time.Millisecond)
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
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
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}
	if depErr.Reason != ReasonNon2XX {
		t.Fatalf("reason=%q want %q", depErr.Reason, ReasonNon2XX)
	}
	if depErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", depErr.Status)
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
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
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
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonMalformedResponse {
		t.Fatalf("expected malformed response, got %v", depErr)
	}
}

func TestClientNegativeCountRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":1,"countedShipments":1,"byStatus":{"IN_TRANSIT":-1},"complete":true}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonInvalidContract {
		t.Fatalf("expected invalid contract, got %v", depErr)
	}
}

func TestClientSumMismatchRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":3,"countedShipments":3,"byStatus":{"IN_TRANSIT":1,"DELIVERED":1},"complete":true}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonInvalidContract {
		t.Fatalf("expected invalid contract for sum mismatch, got %v", depErr)
	}
}

func TestClientCompleteTotalMismatchRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":5,"countedShipments":2,"byStatus":{"IN_TRANSIT":2},"complete":true}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonInvalidContract {
		t.Fatalf("expected invalid contract for total/counted mismatch, got %v", depErr)
	}
}

func TestClientIncompleteAggregateRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"totalShipments":2,"countedShipments":2,"byStatus":{"IN_TRANSIT":2},"complete":false}`))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr == nil || depErr.Reason != ReasonIncomplete {
		t.Fatalf("expected incomplete aggregate, got %v", depErr)
	}
}

func TestClientContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := testClient(t, server.URL, time.Second)
	_, depErr := client.FetchStatusSummary(ctx, "primary", "11111111-1111-1111-1111-111111111111", "req-1")
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
	_, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
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
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	defer server.Close()

	client := testClient(t, server.URL, time.Second)
	payload, depErr := client.FetchStatusSummary(context.Background(), "primary", "11111111-1111-1111-1111-111111111111", "req-1")
	if depErr != nil {
		t.Fatalf("unexpected error: %v", depErr)
	}
	if payload.TotalShipments != 2 || payload.CountedShipments != 2 {
		t.Fatalf("totals=%d/%d want 2/2", payload.TotalShipments, payload.CountedShipments)
	}
	if payload.ByStatus["IN_TRANSIT"] != 1 || payload.ByStatus["DELIVERED"] != 1 {
		t.Fatalf("byStatus=%v", payload.ByStatus)
	}
	if !payload.Complete {
		t.Fatal("expected complete=true")
	}
	var roundTrip map[string]any
	if err := json.Unmarshal([]byte(validAggregateJSON()), &roundTrip); err != nil {
		t.Fatalf("fixture json: %v", err)
	}
}
