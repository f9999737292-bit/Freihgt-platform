package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/freight-platform/analytics-service/internal/config"
)

const tenantA = "11111111-1111-1111-1111-111111111111"

func TestClientValidatesSource(t *testing.T) {
	var gotName, gotTenant, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path != "/internal/v1/analytics/operations-foundation" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Fatal("source call must not use a public query tenant")
		}
		gotName = r.Header.Get("X-Internal-Service-Name")
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotToken = r.Header.Get("X-Internal-Service-Token")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenantId": tenantA, "shipmentTotal": 4, "onTimeDeliveryDenominator": 2,
			"onTimeDeliveryNumerator": 2, "returnCaseCount": 1, "redirectCaseCount": 0,
		})
	}))
	defer server.Close()
	client := NewClient(config.Config{ShipmentURL: server.URL, InternalToken: "secret", SourceTimeout: time.Second})
	snap, err := client.Fetch(context.Background(), tenantA)
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "analytics-service" || gotTenant != tenantA || gotToken != "secret" || snap.ShipmentTotal != 4 {
		t.Fatalf("headers name=%s tenant=%s token=%s snap=%+v", gotName, gotTenant, gotToken, snap)
	}
	if err := client.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClientFailClosed(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   any
		delay  time.Duration
		reason string
	}{
		{name: "http 500", status: http.StatusInternalServerError, body: map[string]any{}, reason: "http_5xx"},
		{name: "malformed", status: http.StatusOK, body: "not-json", reason: "malformed"},
		{name: "wrong tenant", status: http.StatusOK, body: map[string]any{"tenantId": "22222222-2222-2222-2222-222222222222", "shipmentTotal": 1}, reason: "wrong_tenant"},
		{name: "inconsistent", status: http.StatusOK, body: map[string]any{"tenantId": tenantA, "shipmentTotal": 1, "onTimeDeliveryDenominator": 2, "onTimeDeliveryNumerator": 1}, reason: "inconsistent"},
		{name: "timeout", status: http.StatusOK, delay: 200 * time.Millisecond, body: map[string]any{"tenantId": tenantA}, reason: "timeout"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.delay > 0 {
					time.Sleep(tt.delay)
				}
				w.WriteHeader(tt.status)
				switch body := tt.body.(type) {
				case string:
					_, _ = w.Write([]byte(body))
				default:
					_ = json.NewEncoder(w).Encode(body)
				}
			}))
			defer server.Close()
			client := NewClient(config.Config{ShipmentURL: server.URL, InternalToken: "secret", SourceTimeout: 30 * time.Millisecond})
			_, err := client.Fetch(context.Background(), tenantA)
			sourceErr, ok := err.(*Error)
			if !ok || sourceErr.Reason != tt.reason {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
