package identityguard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestProbePlatformAdminLookupRejectsForeignRedirect(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("authorization reached the foreign target")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	verifier := &HTTPVerifier{baseURL: origin.URL, client: &http.Client{}}
	_, err := verifier.IsPlatformAdmin(context.Background(), "Bearer secret-token", "user-a", "tenant-a")
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("error=%v", err)
	}
	if forbidden.Load() != 0 {
		t.Fatalf("forbidden requests=%d", forbidden.Load())
	}
}
