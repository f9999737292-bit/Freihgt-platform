package company

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
)

func TestFC22G_BatchGetCompanyDisplayChunksAt500(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req batchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(req.CompanyIDs) > batchSize {
			t.Fatalf("batch exceeded size: %d", len(req.CompanyIDs))
		}
		items := make([]batchItem, len(req.CompanyIDs))
		for i, id := range req.CompanyIDs {
			items[i] = batchItem{CompanyID: id, LegalName: "Co " + id, Status: "ACTIVE"}
		}
		_ = json.NewEncoder(w).Encode(batchResponse{Items: items})
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token", fcmetrics.New())
	ids := make([]uuid.UUID, 750)
	for i := range ids {
		ids[i] = uuid.New()
	}
	result, err := client.BatchGetCompanyDisplay(context.Background(), uuid.New(), ids)
	if err != nil {
		t.Fatalf("batch get: %v", err)
	}
	if len(result) != len(ids) {
		t.Fatalf("expected %d companies got %d", len(ids), len(result))
	}
	if got := int(calls.Load()); got != 2 {
		t.Fatalf("expected 2 batch HTTP calls for 750 ids, got %d", got)
	}
}

func TestFC22G_BatchGetCompanyDisplayEmpty(t *testing.T) {
	client := NewClient("", "token", fcmetrics.New())
	result, err := client.BatchGetCompanyDisplay(context.Background(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("empty batch: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map")
	}
}

func TestFC22G_BatchGetCompanyDisplayReadsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Fatal("expected request body")
		}
		_ = json.NewEncoder(w).Encode(batchResponse{Items: []batchItem{{
			CompanyID: uuid.NewString(), LegalName: "One", Status: "ACTIVE",
		}}})
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "token", fcmetrics.New())
	_, err := client.BatchGetCompanyDisplay(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
}
