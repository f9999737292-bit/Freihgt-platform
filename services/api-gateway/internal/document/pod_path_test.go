package document

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePODUploadStaysInternal(t *testing.T) {
	var gotPath, gotTenant string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uploadId":"u1"}`))
	}))
	t.Cleanup(upstream.Close)

	client := NewClient(upstream.Client(), upstream.URL)
	_, status, err := client.CreatePODUpload(context.Background(), CreatePODUploadRequest{
		TenantID: "tenant-a", ShipmentID: "s1", DriverID: "d1", OwnerCompanyID: "c1",
		MimeType: "application/pdf", FileName: "pod.pdf", IdempotencyKey: "k1",
	})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if gotPath != "/internal/v1/pod-uploads" || gotTenant != "tenant-a" {
		t.Fatalf("path=%s tenant=%s", gotPath, gotTenant)
	}
}
