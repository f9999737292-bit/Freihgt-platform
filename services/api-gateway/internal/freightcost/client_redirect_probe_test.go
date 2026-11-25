package freightcost

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestProbeFreightCostRejectsForeignRedirect(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	client := NewClient(&http.Client{}, origin.URL, "internal-secret")
	_, _, err := client.Forward(context.Background(), ForwardInput{
		Method:       http.MethodGet,
		InternalPath: "/internal/v1/costs",
		TenantID:     "tenant-a",
		UserID:       "user-a",
		CompanyID:    "company-a",
		ActorKind:    "shipper",
		RequestID:    "req-a",
	})
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("error=%v", err)
	}
	if forbidden.Load() != 0 {
		t.Fatalf("forbidden requests=%d", forbidden.Load())
	}
}
