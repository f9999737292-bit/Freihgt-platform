package routeauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestProbeIdentityRedirectDropsTrustedHeaders(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/steal", http.StatusPermanentRedirect)
	}))
	t.Cleanup(origin.Close)

	client := NewIdentityClient(&http.Client{}, origin.URL)
	_, err := client.FetchUserRoles(context.Background(), RequestContext{
		TenantID:  "tenant-a",
		UserID:    "user-a",
		AuthToken: "Bearer secret-token",
		RequestID: "req-a",
	})
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("error=%v", err)
	}
	if forbidden.Load() != 0 {
		t.Fatalf("forbidden requests=%d", forbidden.Load())
	}
}
