package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freight-platform/shared-go/clientip"
)

func TestRateLimitIgnoresSpoofedForwardedHeaders(t *testing.T) {
	resolver := clientip.NewResolver(nil)
	handler := clientip.CapturePeerMiddleware(
		clientip.StripSpoofableForwardedHeaders(resolver)(
			RateLimit(true, 1, 1, "api-gateway", resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set(clientip.HeaderXForwardedFor, "203.0.113.99")

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}

	reqSpoof := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	reqSpoof.RemoteAddr = "203.0.113.10:12345"
	reqSpoof.Header.Set(clientip.HeaderXForwardedFor, "203.0.113.99")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, reqSpoof)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("same peer with spoofed XFF must stay rate limited: status=%d want 429", rec2.Code)
	}
}
