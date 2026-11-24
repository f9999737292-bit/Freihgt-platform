package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestProbeIntegrationAuthRedirectDoesNotUseDefaultClientPolicy(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		if r.Header.Get("X-Internal-Service-Token") != "" {
			t.Error("internal token reached the foreign target")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	_, err := verifyAPIKeyViaIdentity(context.Background(), IntegrationAuthConfig{
		IdentityInternalURL:  origin.URL,
		InternalServiceToken: "internal-secret",
	}, "api-key", "127.0.0.1")
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("error=%v", err)
	}
	if forbidden.Load() != 0 {
		t.Fatalf("forbidden requests=%d", forbidden.Load())
	}
	if http.DefaultClient.CheckRedirect != nil {
		t.Fatal("http.DefaultClient.CheckRedirect was changed")
	}
}
