package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

const (
	spoofedAllowedIP = "203.0.113.50"
	directPeerIP     = "198.51.100.10"
)

func TestRealIPPoisionsResolverWithoutPeerCapture(t *testing.T) {
	resolver := NewResolver(nil)
	var resolved string
	handler := chimiddleware.RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := resolver.ClientIP(r)
		if err != nil {
			t.Fatalf("client ip: %v", err)
		}
		resolved = ip
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/oauth/token", nil)
	req.RemoteAddr = directPeerIP + ":1234"
	req.Header.Set(HeaderXForwardedFor, spoofedAllowedIP)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if resolved != spoofedAllowedIP {
		t.Fatalf("PRE_FIX exploit not reproduced: resolved=%q want spoofed %q", resolved, spoofedAllowedIP)
	}
}

func TestPeerCaptureIgnoresRealIPPoisoning(t *testing.T) {
	resolver := NewResolver(nil)
	var resolved string
	handler := CapturePeerMiddleware(chimiddleware.RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := resolver.ClientIP(r)
		if err != nil {
			t.Fatalf("client ip: %v", err)
		}
		resolved = ip
	})))

	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/oauth/token", nil)
	req.RemoteAddr = directPeerIP + ":1234"
	req.Header.Set(HeaderXForwardedFor, spoofedAllowedIP)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if resolved != directPeerIP {
		t.Fatalf("resolved=%q want direct peer %q", resolved, directPeerIP)
	}
}

func TestStripSpoofableForwardedHeadersBeforeRealIP(t *testing.T) {
	resolver := NewResolver(nil)
	var resolved string
	handler := CapturePeerMiddleware(
		StripSpoofableForwardedHeaders(resolver)(
			chimiddleware.RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ip, err := resolver.ClientIP(r)
				if err != nil {
					t.Fatalf("client ip: %v", err)
				}
				resolved = ip
			})),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.RemoteAddr = directPeerIP + ":1234"
	req.Header.Set(HeaderXForwardedFor, spoofedAllowedIP)
	req.Header.Set(HeaderXRealIP, spoofedAllowedIP)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if resolved != directPeerIP {
		t.Fatalf("resolved=%q want direct peer %q", resolved, directPeerIP)
	}
}
