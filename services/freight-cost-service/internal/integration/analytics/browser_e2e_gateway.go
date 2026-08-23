//go:build integration

package analytics

import (
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

func startBrowserGatewayProxy(t *testing.T, freightCostURL string, fix browserFixture) (string, *http.Server) {
	t.Helper()
	fcBase, err := url.Parse(freightCostURL)
	if err != nil {
		t.Fatalf("parse freight cost url: %v", err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validateBrowserJWT(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.TrimSpace(r.Header.Get("X-Company-ID")) != fix.BuyerID.String() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		targetPath := strings.Replace(r.URL.Path, "/api/v1/freight-costs", "/internal/v1/freight-costs", 1)
		target, _ := url.Parse(fcBase.String() + targetPath)
		target.RawQuery = r.URL.Query().Encode()

		proxy := httputil.NewSingleHostReverseProxy(fcBase)
		proxy.Director = func(req *http.Request) {
			req.URL = target
			req.Host = fcBase.Host
			req.Header.Set("X-Internal-Service-Token", browserE2EInternalToken)
			req.Header.Set("X-Tenant-ID", fix.TenantID.String())
			req.Header.Set("X-User-ID", fix.UserID.String())
			req.Header.Set("X-Company-ID", fix.BuyerID.String())
			req.Header.Set("X-Actor-Kind", "BUYER")
		}
		proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(rw, "bad gateway", http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	})

	r := chi.NewRouter()
	r.Get("/api/v1/freight-costs/analytics/overview", handler.ServeHTTP)
	r.Get("/api/v1/freight-costs/analytics/lanes", handler.ServeHTTP)
	r.Get("/api/v1/freight-costs/analytics/carriers", handler.ServeHTTP)
	r.Get("/api/v1/freight-costs/analytics/accessorials", handler.ServeHTTP)
	r.Get("/api/v1/freight-costs/opportunities", handler.ServeHTTP)
	_ = io.Discard
	return listenHTTPServer(t, r)
}

func validateBrowserJWT(r *http.Request) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return []byte(browserE2EJWTSecret), nil
	})
	return err == nil && token.Valid
}
