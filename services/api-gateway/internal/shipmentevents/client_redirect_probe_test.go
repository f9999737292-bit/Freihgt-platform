package shipmentevents

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
)

func TestProbeShipmentEventsRejectsForeignRedirect(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/steal", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	client := NewDownstreamClient(&http.Client{}, origin.URL, origin.URL, origin.URL, origin.URL, 20)
	_, err := client.FetchUserRoles(context.Background(), RequestContext{
		TenantID:  "tenant-a",
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

func TestProbeShipmentEventsDocumentRedirectStaysOffShipmentOrigin(t *testing.T) {
	t.Parallel()
	var shipmentHits atomic.Int32
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shipmentHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(shipment.Close)

	documents := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, shipment.URL+"/v1/shipments", http.StatusFound)
	}))
	t.Cleanup(documents.Close)

	shared := &http.Client{}
	client := NewDownstreamClient(shared, "http://identity.internal", shipment.URL, documents.URL, "http://billing.internal", 20)
	_, _, err := client.FetchDocuments(context.Background(), RequestContext{TenantID: "tenant-a", AuthToken: "Bearer secret-token"}, "s1")
	if !errors.Is(err, httpredirect.ErrRejected) {
		t.Fatalf("error=%v", err)
	}
	if shipmentHits.Load() != 0 {
		t.Fatalf("shipment requests=%d", shipmentHits.Load())
	}
	if shared.CheckRedirect != nil {
		t.Fatal("shared CheckRedirect was set")
	}
}
