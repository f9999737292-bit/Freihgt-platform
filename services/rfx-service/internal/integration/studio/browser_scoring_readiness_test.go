//go:build integration

package studio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	return id
}

func TestScoringModelGatewayReady_PassesOn200(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		if calls.Load() < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL","message":"warming up"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":{"status":"DRAFT"}}`))
	}))
	t.Cleanup(srv.Close)

	fix := browserScoringFixture{
		browserStudioFixture: browserStudioFixture{
			EventID:   mustUUID(t, "2882c984-7eae-45d5-8f05-c1a99c739e5e"),
			CompanyID: mustUUID(t, "11111111-1111-1111-1111-111111111111"),
			JWT:       "test-jwt",
		},
	}
	if err := scoringModelGatewayReady(context.Background(), srv.URL, fix, 5*time.Second); err != nil {
		t.Fatalf("expected readiness pass: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected retry after transient 500, calls=%d", calls.Load())
	}
}

func TestScoringModelGatewayReady_FailsFastOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"denied"}}`))
	}))
	t.Cleanup(srv.Close)

	fix := browserScoringFixture{
		browserStudioFixture: browserStudioFixture{
			EventID:   mustUUID(t, "2882c984-7eae-45d5-8f05-c1a99c739e5e"),
			CompanyID: mustUUID(t, "11111111-1111-1111-1111-111111111111"),
			JWT:       "test-jwt",
		},
	}
	err := scoringModelGatewayReady(context.Background(), srv.URL, fix, 2*time.Second)
	if err == nil {
		t.Fatal("expected non-retryable readiness failure")
	}
	if !strings.Contains(err.Error(), "non-retryable") {
		t.Fatalf("unexpected error: %v", err)
	}
}
