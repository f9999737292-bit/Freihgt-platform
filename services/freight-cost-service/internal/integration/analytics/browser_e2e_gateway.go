//go:build integration

package analytics

import (
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
	proxyHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			writeBrowserGatewayCORS(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeBrowserGatewayCORS(w, r)
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
		proxy.ModifyResponse = func(resp *http.Response) error {
			applyBrowserGatewayCORS(resp.Header, r)
			return nil
		}
		proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(rw, "bad gateway", http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	}

	summaryProbeHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			writeBrowserGatewayCORS(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeBrowserGatewayCORS(w, r)
		if !validateBrowserJWT(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order_count":0,"planned_total":"0","current_actual_total":"0","currency_code":"RUB","data_quality":"OK"}`))
	}

	r := chi.NewRouter()
	r.HandleFunc("/api/v1/freight-costs/summary", summaryProbeHandler)
	r.HandleFunc("/api/v1/freight-costs/analytics/overview", proxyHandler)
	r.HandleFunc("/api/v1/freight-costs/analytics/lanes", proxyHandler)
	r.HandleFunc("/api/v1/freight-costs/analytics/carriers", proxyHandler)
	r.HandleFunc("/api/v1/freight-costs/analytics/accessorials", proxyHandler)
	r.HandleFunc("/api/v1/freight-costs/opportunities", proxyHandler)
	return listenHTTPServer(t, r)
}

func writeBrowserGatewayCORS(w http.ResponseWriter, r *http.Request) {
	applyBrowserGatewayCORS(w.Header(), r)
}

func applyBrowserGatewayCORS(h http.Header, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = "*"
	}
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Vary", "Origin")
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Allow-Headers", "Authorization, Accept, Content-Type, X-Company-ID, X-Tenant-ID, X-User-ID, X-Locale, X-Request-ID")
	h.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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
