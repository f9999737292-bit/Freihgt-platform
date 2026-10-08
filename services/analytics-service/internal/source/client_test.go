package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		if r.URL.Path != "/internal/v1/analytics/operations-foundation-v2" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Fatal("source call must not use a public query tenant")
		}
		gotName = r.Header.Get("X-Internal-Service-Name")
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotToken = r.Header.Get("X-Internal-Service-Token")
		_ = json.NewEncoder(w).Encode(completeSource(tenantA, 4, 2, 2, 1, 0))
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

func TestMissingRequiredSourceFieldsFailClosed(t *testing.T) {
	complete := completeSource(tenantA, 0, 0, 0, 0, 0)
	fields := []string{
		"tenantId",
		"shipmentTotal",
		"onTimePickupDenominator",
		"onTimePickupNumerator",
		"onTimeDeliveryDenominator",
		"onTimeDeliveryNumerator",
		"returnCaseCount",
		"redirectCaseCount",
		"carriers",
	}
	for _, field := range fields {
		t.Run(field+" omitted", func(t *testing.T) {
			assertMalformed(t, cloneWithout(t, complete, field))
		})
	}
	t.Run("explicit zeros", func(t *testing.T) {
		raw, err := json.Marshal(complete)
		if err != nil {
			t.Fatal(err)
		}
		snap, err := decodeSnapshot(raw, tenantA)
		if err != nil {
			t.Fatal(err)
		}
		if snap.ShipmentTotal != 0 || snap.OnTimeDeliveryDenominator != 0 || snap.OnTimeDeliveryNumerator != 0 || snap.ReturnCaseCount != 0 || snap.RedirectCaseCount != 0 {
			t.Fatalf("%+v", snap)
		}
	})
	raw, err := json.Marshal(complete)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("trailing second document", func(t *testing.T) {
		assertMalformed(t, append(append([]byte{}, raw...), []byte(`{"shipmentTotal":1}`)...))
	})
	t.Run("trailing non-whitespace", func(t *testing.T) {
		assertMalformed(t, append(append([]byte{}, raw...), []byte(" trailing")...))
	})
}

func completeSource(tenantID string, shipmentTotal, denominator, numerator, returns, redirects int64) map[string]any {
	return map[string]any{
		"tenantId":                  tenantID,
		"shipmentTotal":             shipmentTotal,
		"onTimePickupDenominator":   int64(0),
		"onTimePickupNumerator":     int64(0),
		"onTimeDeliveryDenominator": denominator,
		"onTimeDeliveryNumerator":   numerator,
		"returnCaseCount":           returns,
		"redirectCaseCount":         redirects,
		"carriers":                  []any{},
	}
}

func cloneWithout(t *testing.T, source map[string]any, field string) []byte {
	t.Helper()
	cloned := make(map[string]any, len(source)-1)
	for key, value := range source {
		if key == field {
			continue
		}
		cloned[key] = value
	}
	raw, err := json.Marshal(cloned)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAN03BSourceCarrierValidation(t *testing.T) {
	carrierA := "22222222-2222-2222-2222-222222222222"
	carrierB := "33333333-3333-3333-3333-333333333333"
	base := completeSource(tenantA, 10, 4, 2, 0, 0)
	base["onTimePickupDenominator"] = int64(4)
	base["onTimePickupNumerator"] = int64(2)
	base["carriers"] = []any{
		carrierRow(carrierB, 1, 1, 1, 1),
		carrierRow(carrierA, 1, 0, 1, 0),
	}
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("AN03B_10 source order preserved until KPI sort", func(t *testing.T) {
		snap, err := decodeSnapshot(raw, tenantA)
		if err != nil || len(snap.Carriers) != 2 || snap.Carriers[0].CarrierCompanyID != carrierB {
			t.Fatalf("%+v %v", snap.Carriers, err)
		}
	})
	t.Run("AN03B_12", func(t *testing.T) {
		body := cloneMap(t, base)
		body["carriers"] = []any{carrierRow(carrierA, 1, 0, 1, 0), carrierRow(strings.ToUpper(carrierA), 1, 0, 1, 0)}
		assertReason(t, mustJSON(t, body), "inconsistent")
	})
	t.Run("AN03B_13", func(t *testing.T) {
		body := cloneMap(t, base)
		body["carriers"] = []any{carrierRow(carrierA, 1, 2, 1, 0)}
		assertReason(t, mustJSON(t, body), "inconsistent")
	})
	t.Run("AN03B_14", func(t *testing.T) {
		body := cloneMap(t, base)
		body["carriers"] = []any{carrierRow(carrierA, 5, 2, 1, 0)}
		assertReason(t, mustJSON(t, body), "inconsistent")
	})
	t.Run("AN03B_15", func(t *testing.T) {
		assertReason(t, mustJSON(t, completeSource("22222222-2222-2222-2222-222222222222", 1, 0, 0, 0, 0)), "wrong_tenant")
	})
	t.Run("AN03B_16", func(t *testing.T) {
		assertMalformed(t, cloneWithout(t, base, "onTimePickupNumerator"))
	})
	t.Run("AN03B_17", func(t *testing.T) {
		body := cloneMap(t, base)
		body["latePickupCount"] = int64(1)
		assertMalformed(t, mustJSON(t, body))
	})
	t.Run("AN03B_18", func(t *testing.T) {
		assertMalformed(t, append(append([]byte{}, raw...), []byte(`{"extra":1}`)...))
	})
	t.Run("zero carrier id", func(t *testing.T) {
		body := cloneMap(t, base)
		body["carriers"] = []any{carrierRow("00000000-0000-0000-0000-000000000000", 0, 0, 0, 0)}
		assertReason(t, mustJSON(t, body), "inconsistent")
	})
}

func carrierRow(id string, pickupDenominator, pickupNumerator, deliveryDenominator, deliveryNumerator int64) map[string]any {
	return map[string]any{
		"carrierCompanyId":          id,
		"onTimePickupDenominator":   pickupDenominator,
		"onTimePickupNumerator":     pickupNumerator,
		"onTimeDeliveryDenominator": deliveryDenominator,
		"onTimeDeliveryNumerator":   deliveryNumerator,
	}
}

func cloneMap(t *testing.T, source map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func mustJSON(t *testing.T, body map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertReason(t *testing.T, body []byte, reason string) {
	t.Helper()
	_, err := decodeSnapshot(body, tenantA)
	sourceErr, ok := err.(*Error)
	if !ok || sourceErr.Reason != reason {
		t.Fatalf("err=%v body=%s", err, body)
	}
}

func assertMalformed(t *testing.T, body []byte) {
	t.Helper()
	_, err := decodeSnapshot(body, tenantA)
	sourceErr, ok := err.(*Error)
	if !ok || sourceErr.Reason != "malformed" {
		t.Fatalf("err=%v body=%s", err, body)
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
		{name: "wrong tenant", status: http.StatusOK, body: completeSource("22222222-2222-2222-2222-222222222222", 1, 0, 0, 0, 0), reason: "wrong_tenant"},
		{name: "inconsistent", status: http.StatusOK, body: completeSource(tenantA, 1, 2, 1, 0, 0), reason: "inconsistent"},
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
