package document

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestProbeDocumentUploadRedirectDoesNotForwardBody(t *testing.T) {
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

	client := NewClient(&http.Client{}, origin.URL)
	_, _, err := client.CreatePODUpload(context.Background(), CreatePODUploadRequest{
		TenantID:       "tenant-a",
		ShipmentID:     "s1",
		DriverID:       "d1",
		OwnerCompanyID: "c1",
		MimeType:       "application/pdf",
		FileName:       "pod.pdf",
		IdempotencyKey: "idem-a",
	})
	if err == nil {
		t.Fatal("expected rejected redirect")
	}
	if forbidden.Load() != 0 {
		t.Fatalf("forbidden requests=%d", forbidden.Load())
	}
}
