//go:build integration

package controltowerreadmodelintegration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	gatewayrm "github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func TestBlackBoxPrimarySuccess(t *testing.T) {
	h := newReadModelHarness(t)
	tenantA := uuid.New()
	h.seedProjections(tenantA, []string{"IN_TRANSIT", "IN_TRANSIT", "DELIVERED"})

	client := h.client(2 * time.Second)
	payload, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, tenantA.String(), "bb-primary")
	if depErr != nil {
		t.Fatalf("fetch failed: %v", depErr)
	}
	if payload.TotalShipments != 3 {
		t.Fatalf("total=%d want 3", payload.TotalShipments)
	}
	if payload.ByStatus["IN_TRANSIT"] != 2 || payload.ByStatus["DELIVERED"] != 1 {
		t.Fatalf("byStatus=%v", payload.ByStatus)
	}

	mergeOut := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode: gatewayrm.ModePrimary,
		Legacy: gatewayrm.LegacyStatusInput{
			TotalShipments:   999,
			CountedShipments: 1,
			ByStatus:         map[string]int64{"CANCELLED": 1},
			LimitedDataset:   true,
		},
		ReadModel:              payload,
		RequireConsumerRunning: false,
	})
	if mergeOut.StatusSummary == nil || mergeOut.StatusSummary.Source != gatewayrm.SourceReadModel {
		t.Fatalf("merge source=%q", mergeOut.StatusSummary.Source)
	}
	if mergeOut.StatusSummaryFreshness.FallbackUsed || mergeOut.StatusSummaryFreshness.Partial {
		t.Fatalf("freshness=%+v", mergeOut.StatusSummaryFreshness)
	}
	if mergeOut.StatusSummary.LimitedDataset {
		t.Fatal("read-model summary must not be limited")
	}
}

func TestBlackBoxTenantIsolation(t *testing.T) {
	h := newReadModelHarness(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	h.seedProjections(tenantA, []string{"IN_TRANSIT", "IN_TRANSIT", "DELIVERED"})
	h.seedProjections(tenantB, []string{"IN_TRANSIT", "DELIVERED"})

	client := h.client(2 * time.Second)
	payloadA, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, tenantA.String(), "bb-tenant-a")
	if depErr != nil {
		t.Fatalf("tenant A fetch: %v", depErr)
	}
	if payloadA.TotalShipments != 3 {
		t.Fatalf("tenant A total=%d", payloadA.TotalShipments)
	}

	payloadB, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, tenantB.String(), "bb-tenant-b")
	if depErr != nil {
		t.Fatalf("tenant B fetch: %v", depErr)
	}
	if payloadB.TotalShipments != 2 {
		t.Fatalf("tenant B total=%d want 2", payloadB.TotalShipments)
	}
}

func TestBlackBoxPrimaryFallback503(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(failing.Close)

	client := gatewayrm.NewClient(&http.Client{Timeout: time.Second}, gatewayrm.Config{
		BaseURL:          failing.URL,
		Timeout:          time.Second,
		MaxResponseBytes: 256 * 1024,
	}, gatewayrm.NewMetrics())

	_, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, uuid.NewString(), "bb-fallback-503")
	if depErr == nil {
		t.Fatal("expected dependency error")
	}

	mergeOut := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode: gatewayrm.ModePrimary,
		Legacy: gatewayrm.LegacyStatusInput{
			TotalShipments:   10,
			CountedShipments: 10,
			ByStatus:         map[string]int64{"IN_TRANSIT": 10},
			LimitedDataset:   false,
		},
		ReadModelErr:           depErr,
		RequireConsumerRunning: true,
	})
	if mergeOut.StatusSummary.Source != gatewayrm.SourceLegacy || !mergeOut.StatusSummaryFreshness.FallbackUsed {
		t.Fatalf("expected legacy fallback, got %+v", mergeOut.StatusSummary)
	}
	if !strings.Contains(strings.Join(mergeOut.Warnings, ","), gatewayrm.WarningUnavailable) {
		t.Fatalf("warnings=%v", mergeOut.Warnings)
	}
}

func TestBlackBoxPrimaryFallbackTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"totalShipments":0,"byStatus":{},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
	}))
	t.Cleanup(slow.Close)

	client := gatewayrm.NewClient(&http.Client{Timeout: 100 * time.Millisecond}, gatewayrm.Config{
		BaseURL:          slow.URL,
		Timeout:          100 * time.Millisecond,
		MaxResponseBytes: 256 * 1024,
	}, gatewayrm.NewMetrics())

	_, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, uuid.NewString(), "bb-fallback-timeout")
	if depErr == nil || depErr.Reason != gatewayrm.ReasonTimeout {
		t.Fatalf("expected timeout, got %v", depErr)
	}
}

func TestBlackBoxPrimaryFallbackMalformedJSON(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{invalid"))
	}))
	t.Cleanup(bad.Close)

	client := gatewayrm.NewClient(&http.Client{Timeout: time.Second}, gatewayrm.Config{
		BaseURL:          bad.URL,
		Timeout:          time.Second,
		MaxResponseBytes: 256 * 1024,
	}, gatewayrm.NewMetrics())

	_, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, uuid.NewString(), "bb-fallback-malformed")
	if depErr == nil || depErr.Reason != gatewayrm.ReasonMalformedResponse {
		t.Fatalf("expected malformed response, got %v", depErr)
	}
}

func TestBlackBoxShadowResponseUnchanged(t *testing.T) {
	h := newReadModelHarness(t)
	tenantA := uuid.New()
	h.seedProjections(tenantA, []string{"IN_TRANSIT", "IN_TRANSIT", "DELIVERED"})

	client := h.client(2 * time.Second)
	payload, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModeShadow, tenantA.String(), "bb-shadow")
	if depErr != nil {
		t.Fatalf("fetch failed: %v", depErr)
	}

	legacy := gatewayrm.LegacyStatusInput{
		TotalShipments:         3,
		CountedShipments:       3,
		ByStatus:               map[string]int64{"IN_TRANSIT": 2, "DELIVERED": 1},
		LimitedDataset:         false,
		FullAggregateAvailable: true,
	}
	out := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode:                   gatewayrm.ModeShadow,
		Legacy:                 legacy,
		ReadModel:              payload,
		RequireConsumerRunning: false,
	})
	if out.StatusSummary == nil || out.StatusSummary.Source != gatewayrm.SourceLegacy {
		t.Fatalf("shadow merge must expose legacy status summary, got %+v", out.StatusSummary)
	}
	if out.Comparison != gatewayrm.ComparisonMatch {
		t.Fatalf("comparison=%q want MATCH", out.Comparison)
	}
}

func TestBlackBoxLimitedLegacyFallback(t *testing.T) {
	out := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode: gatewayrm.ModePrimary,
		Legacy: gatewayrm.LegacyStatusInput{
			TotalShipments:   1200,
			CountedShipments: 100,
			ByStatus:         map[string]int64{"IN_TRANSIT": 100},
			LimitedDataset:   true,
		},
		ReadModelErr:           &gatewayrm.DependencyError{Reason: gatewayrm.ReasonNon2XX, Status: 503},
		RequireConsumerRunning: true,
	})
	if !out.StatusSummary.LimitedDataset || out.StatusSummary.CountedShipments != 100 {
		t.Fatalf("summary=%+v", out.StatusSummary)
	}
	for _, code := range []string{gatewayrm.WarningUnavailable, gatewayrm.WarningFallbackUsed, gatewayrm.WarningLegacyLimited} {
		found := false
		for _, w := range out.Warnings {
			if w == code {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing warning %s in %v", code, out.Warnings)
		}
	}
}

func TestBlackBoxClientHeadersAndNoSecretsInError(t *testing.T) {
	tenantA := uuid.New()
	var gotTenant, gotAuth, gotCookie string
	checkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		_, _ = w.Write([]byte(`{"totalShipments":1,"byStatus":{"IN_TRANSIT":1},"incompleteProjections":0,"freshness":{"consumerRunning":true}}`))
	}))
	t.Cleanup(checkServer.Close)

	client := gatewayrm.NewClient(&http.Client{Timeout: time.Second}, gatewayrm.Config{
		BaseURL:          checkServer.URL,
		Timeout:          time.Second,
		MaxResponseBytes: 256 * 1024,
	}, gatewayrm.NewMetrics())
	_, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, tenantA.String(), "bb-headers")
	if depErr != nil {
		t.Fatalf("unexpected error: %v", depErr)
	}
	if gotTenant != tenantA.String() {
		t.Fatalf("tenant=%q", gotTenant)
	}
	if gotAuth != "" || gotCookie != "" {
		t.Fatalf("unexpected auth/cookie headers")
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"secret":"internal","totalShipments":-1}`))
	}))
	t.Cleanup(bad.Close)
	client2 := gatewayrm.NewClient(&http.Client{Timeout: time.Second}, gatewayrm.Config{
		BaseURL:          bad.URL,
		Timeout:          time.Second,
		MaxResponseBytes: 256 * 1024,
	}, gatewayrm.NewMetrics())
	_, depErr2 := client2.FetchStatusSummary(context.Background(), gatewayrm.ModePrimary, tenantA.String(), "bb-secret")
	if depErr2 == nil {
		t.Fatal("expected contract error")
	}
	if strings.Contains(depErr2.Error(), "internal") {
		t.Fatalf("raw body leaked: %q", depErr2.Error())
	}

	var payload map[string]any
	_ = json.Unmarshal([]byte(`{"tenantId":"hidden"}`), &payload)
}

func TestBlackBoxShadowLimitedLegacyComparison(t *testing.T) {
	h := newReadModelHarness(t)
	tenantA := uuid.New()
	h.seedProjections(tenantA, []string{"IN_TRANSIT", "IN_TRANSIT", "DELIVERED"})

	client := h.client(2 * time.Second)
	payload, depErr := client.FetchStatusSummary(context.Background(), gatewayrm.ModeShadow, tenantA.String(), "bb-shadow-limited")
	if depErr != nil {
		t.Fatalf("fetch failed: %v", depErr)
	}

	out := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode: gatewayrm.ModeShadow,
		Legacy: gatewayrm.LegacyStatusInput{
			TotalShipments:   1200,
			CountedShipments: 100,
			ByStatus:         map[string]int64{"IN_TRANSIT": 100},
			LimitedDataset:   true,
		},
		ReadModel:              payload,
		RequireConsumerRunning: false,
	})
	if out.Comparison != gatewayrm.ComparisonLegacyLimitedDataset {
		t.Fatalf("comparison=%q want LEGACY_LIMITED_DATASET", out.Comparison)
	}
}
