package legacyaggregate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestProbeLegacyAggregateRejectsForeignRedirect(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	t.Cleanup(foreign.Close)

	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			forbidden.Store(0)
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, foreign.URL+"/steal", status)
			}))
			defer origin.Close()

			client := testClient(t, origin.URL, time.Second)
			_, depErr := client.FetchStatusSummary(context.Background(), "primary", "tenant-a", "req-a")
			if depErr == nil || !errors.Is(depErr.Err, httpredirect.ErrRejected) {
				t.Fatalf("error=%v", depErr)
			}
			if forbidden.Load() != 0 {
				t.Fatalf("forbidden requests=%d", forbidden.Load())
			}
			if strings.Contains(depErr.Error(), "tenant-a") || strings.Contains(depErr.Error(), foreign.URL) {
				t.Fatalf("error leaked details: %v", depErr)
			}
		})
	}
}

func TestProbeLegacyAggregateSameOriginRestoresTenant(t *testing.T) {
	t.Parallel()
	var gotTenant, gotRequest string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == statusSummaryPath {
			http.Redirect(w, r, "http://"+r.Host+"/internal/v1/shipments/status-summary-final", http.StatusFound)
			return
		}
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotRequest = r.Header.Get("X-Request-ID")
		_, _ = w.Write([]byte(validAggregateJSON()))
	}))
	defer origin.Close()

	caller := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 && via[0] != nil {
				via[0].Header.Set("X-Tenant-ID", "attacker-tenant")
				via[0].Header.Set("X-Request-ID", "attacker-req")
			}
			req.Header.Set("X-Tenant-ID", "attacker-tenant")
			req.Header.Set("X-Request-ID", "attacker-req")
			req.Host = "attacker.internal"
			return nil
		},
	}
	client := NewClient(caller, Config{BaseURL: origin.URL, Timeout: time.Second, MaxResponseBytes: 256 * 1024}, NewMetrics())
	summary, depErr := client.FetchStatusSummary(context.Background(), "primary", "tenant-a", "req-a")
	if depErr != nil {
		t.Fatal(depErr)
	}
	if summary == nil || !summary.Complete {
		t.Fatalf("summary=%v", summary)
	}
	if gotTenant != "tenant-a" || gotRequest != "req-a" {
		t.Fatalf("tenant=%q request=%q", gotTenant, gotRequest)
	}
	if caller.CheckRedirect == nil {
		t.Fatal("caller CheckRedirect was removed")
	}
}
