package controltower

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestProbeDownstreamRejectsForeignRedirect(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			forbidden.Store(0)
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, foreign.URL+"/steal", status)
			}))
			defer origin.Close()

			client := NewDownstreamClient(http.DefaultClient, origin.URL, origin.URL, origin.URL, origin.URL, origin.URL, 20)
			_, _, err := client.FetchShipments(context.Background(), RequestContext{
				TenantID:  "tenant-a",
				UserID:    "user-a",
				RequestID: "req-a",
				AuthToken: "Bearer secret-token",
			})
			if !errors.Is(err, httpredirect.ErrRejected) {
				t.Fatalf("error=%v", err)
			}
			if forbidden.Load() != 0 {
				t.Fatalf("forbidden requests=%d", forbidden.Load())
			}
			if http.DefaultClient.CheckRedirect != nil {
				t.Fatal("http.DefaultClient.CheckRedirect was changed")
			}
			if strings.Contains(err.Error(), "tenant-a") || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), foreign.URL) {
				t.Fatalf("error leaked details: %v", err)
			}
		})
	}
}

func TestProbeDownstreamSameOriginRestoresTrustedHeaders(t *testing.T) {
	t.Parallel()
	var gotTenant, gotUser, gotRequest, gotAuth string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/shipments" {
			http.Redirect(w, r, "http://"+r.Host+"/v1/shipments-final", http.StatusFound)
			return
		}
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotRequest = r.Header.Get("X-Request-ID")
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer origin.Close()

	caller := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && via[0] != nil {
			via[0].Header.Set("X-Tenant-ID", "attacker-tenant")
			via[0].Header.Set("Authorization", "Bearer attacker")
		}
		req.Header.Set("X-Tenant-ID", "attacker-tenant")
		req.Header.Set("X-User-ID", "attacker-user")
		req.Header.Set("X-Request-ID", "attacker-req")
		req.Header.Set("Authorization", "Bearer attacker")
		req.Host = "attacker.internal"
		return nil
	}}
	client := NewDownstreamClient(caller, origin.URL, origin.URL, origin.URL, origin.URL, origin.URL, 20)
	_, _, err := client.FetchShipments(context.Background(), RequestContext{
		TenantID:  "tenant-a",
		UserID:    "user-a",
		RequestID: "req-a",
		AuthToken: "Bearer secret-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotTenant != "tenant-a" || gotUser != "user-a" || gotRequest != "req-a" || gotAuth != "Bearer secret-token" {
		t.Fatalf("tenant=%q user=%q request=%q auth=%q", gotTenant, gotUser, gotRequest, gotAuth)
	}
	if caller.CheckRedirect == nil {
		t.Fatal("caller CheckRedirect was removed")
	}
}
