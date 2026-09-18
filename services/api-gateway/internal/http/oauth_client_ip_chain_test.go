package http

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/shared-go/clientip"
)

const (
	allowedCIDRClient = "203.0.113.50"
	deniedDirectPeer  = "198.51.100.10"
	oauthGatewayPath  = "/api/v1/integrations/oauth/token"
)

func newIdentityOAuthTestServer(t *testing.T, allowedCIDR string, trustedProxies string) *httptest.Server {
	t.Helper()
	trusted, err := clientip.ParseTrustedProxyCIDRs(trustedProxies)
	if err != nil {
		t.Fatalf("trusted proxies: %v", err)
	}
	resolver := clientip.NewResolver(trusted)
	_, allowedNet, err := net.ParseCIDR(allowedCIDR)
	if err != nil {
		t.Fatalf("allowed cidr: %v", err)
	}

	r := chi.NewRouter()
	r.Use(clientip.CapturePeerMiddleware)
	r.Use(chimiddleware.RealIP)
	r.Post("/v1/integrations/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		clientIP, err := resolver.ClientIP(r)
		if err != nil {
			writeOAuthChainError(w, http.StatusForbidden, "access_denied", "client ip not allowed")
			return
		}
		ip := net.ParseIP(clientIP)
		if ip == nil || !allowedNet.Contains(ip) {
			writeOAuthChainError(w, http.StatusForbidden, "access_denied", "client ip not allowed")
			return
		}
		if err := r.ParseForm(); err != nil {
			writeOAuthChainError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
			return
		}
		clientID := strings.TrimSpace(r.FormValue("client_id"))
		clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
		if clientID != "valid-client" || clientSecret != "valid-secret" {
			writeOAuthChainError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-token"})
	})

	return httptest.NewServer(r)
}

func writeOAuthChainError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": description,
	})
}

func newGatewayOAuthProxy(t *testing.T, identityURL string, trustedProxies string) http.Handler {
	t.Helper()
	trusted, err := clientip.ParseTrustedProxyCIDRs(trustedProxies)
	if err != nil {
		t.Fatalf("trusted proxies: %v", err)
	}
	resolver := clientip.NewResolver(trusted)
	target, err := url.Parse(identityURL)
	if err != nil {
		t.Fatalf("parse identity url: %v", err)
	}

	cfg := config.Config{
		Services:            config.ServiceURLs{Identity: identityURL},
		ProxyTimeoutSeconds: 30,
	}
	proxyHandler, err := NewProxyHandler(cfg)
	if err != nil {
		t.Fatalf("proxy handler: %v", err)
	}
	proxyHandler.clientIPResolver = resolver
	_ = target

	r := chi.NewRouter()
	r.Use(clientip.CapturePeerMiddleware)
	r.Use(clientip.StripSpoofableForwardedHeaders(resolver))
	r.Use(chimiddleware.RealIP)
	r.Handle("/api/*", proxyHandler)
	return r
}

func postOAuthToken(t *testing.T, handler http.Handler, remoteAddr string, headers map[string]string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, oauthGatewayPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestGatewayIdentityOAuthUntrustedSpoofedXFFDenied(t *testing.T) {
	identity := newIdentityOAuthTestServer(t, "203.0.113.0/24", "127.0.0.1/32")
	defer identity.Close()

	gateway := newGatewayOAuthProxy(t, identity.URL, "")
	rec := postOAuthToken(t, gateway, deniedDirectPeer+":1234", map[string]string{
		clientip.HeaderXForwardedFor: allowedCIDRClient,
	}, "grant_type=client_credentials&client_id=valid-client&client_secret=valid-secret")

	if rec.Code != http.StatusForbidden {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("status=%d want 403 body=%s", rec.Code, body)
	}
}

func TestGatewayIdentityOAuthTrustedProxyAllowedClient(t *testing.T) {
	identity := newIdentityOAuthTestServer(t, "203.0.113.0/24", "127.0.0.1/32")
	defer identity.Close()

	gateway := newGatewayOAuthProxy(t, identity.URL, "127.0.0.1/32")
	rec := postOAuthToken(t, gateway, "127.0.0.1:54321", map[string]string{
		clientip.HeaderXForwardedFor: allowedCIDRClient,
	}, "grant_type=client_credentials&client_id=valid-client&client_secret=valid-secret")

	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("status=%d want 200 body=%s", rec.Code, body)
	}
}

func TestGatewayIdentityOAuthTrustedProxyDeniedClient(t *testing.T) {
	identity := newIdentityOAuthTestServer(t, "203.0.113.0/24", "127.0.0.1/32")
	defer identity.Close()

	gateway := newGatewayOAuthProxy(t, identity.URL, "127.0.0.1/32")
	rec := postOAuthToken(t, gateway, "127.0.0.1:54321", map[string]string{
		clientip.HeaderXForwardedFor: deniedDirectPeer,
	}, "grant_type=client_credentials&client_id=valid-client&client_secret=valid-secret")

	if rec.Code != http.StatusForbidden {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("status=%d want 403 body=%s", rec.Code, body)
	}
}

func TestGatewayIdentityOAuthInvalidCredentials(t *testing.T) {
	identity := newIdentityOAuthTestServer(t, "203.0.113.0/24", "127.0.0.1/32")
	defer identity.Close()

	gateway := newGatewayOAuthProxy(t, identity.URL, "")
	rec := postOAuthToken(t, gateway, allowedCIDRClient+":1234", nil,
		"grant_type=client_credentials&client_id=valid-client&client_secret=wrong-secret")

	if rec.Code != http.StatusUnauthorized {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("status=%d want 401 body=%s", rec.Code, body)
	}
}

func TestGatewayIdentityOAuthValidCredentialsAllowedIP(t *testing.T) {
	identity := newIdentityOAuthTestServer(t, "203.0.113.0/24", "127.0.0.1/32")
	defer identity.Close()

	gateway := newGatewayOAuthProxy(t, identity.URL, "")
	rec := postOAuthToken(t, gateway, allowedCIDRClient+":1234", nil,
		"grant_type=client_credentials&client_id=valid-client&client_secret=valid-secret")

	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("status=%d want 200 body=%s", rec.Code, body)
	}
}

func TestGatewayProxyStripsClientForwardedHeaders(t *testing.T) {
	var gotXFF string
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF = r.Header.Get(clientip.HeaderXForwardedFor)
		w.WriteHeader(http.StatusOK)
	}))
	defer identity.Close()

	gateway := newGatewayOAuthProxy(t, identity.URL, "")
	req := httptest.NewRequest(http.MethodPost, oauthGatewayPath, nil)
	req.RemoteAddr = deniedDirectPeer + ":1234"
	req.Header.Set(clientip.HeaderXForwardedFor, allowedCIDRClient)
	req.Header.Set(clientip.HeaderXRealIP, allowedCIDRClient)
	req.Header.Set(clientip.HeaderForwarded, `for="`+allowedCIDRClient+`"`)
	rec := httptest.NewRecorder()
	gateway.ServeHTTP(rec, req)

	if gotXFF != deniedDirectPeer {
		t.Fatalf("downstream XFF=%q want single canonical value %q", gotXFF, deniedDirectPeer)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("proxy status=%d want 200", rec.Code)
	}
}
