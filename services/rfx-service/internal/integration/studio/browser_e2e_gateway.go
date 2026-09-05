//go:build integration

package studio

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func startBrowserGatewayProxy(t *testing.T, rfxServiceURL string, fix browserStudioFixture) (string, *http.Server) {
	t.Helper()
	rfxBase, err := url.Parse(rfxServiceURL)
	if err != nil {
		t.Fatalf("parse rfx service url: %v", err)
	}

	proxyHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			writeBrowserGatewayCORS(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeBrowserGatewayCORS(w, r)
		claims, ok := validateBrowserJWT(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		targetPath := strings.Replace(r.URL.Path, "/api/v1/rfx-events", "/v1/rfx-events", 1)
		target, _ := url.Parse(rfxBase.String() + targetPath)
		target.RawQuery = r.URL.Query().Encode()

		proxy := httputil.NewSingleHostReverseProxy(rfxBase)
		proxy.Director = func(req *http.Request) {
			req.URL = target
			req.Host = rfxBase.Host
			req.Header.Set("X-Tenant-ID", claims.tenantID)
			req.Header.Set("X-User-ID", claims.userID)
			if company := strings.TrimSpace(r.Header.Get("X-Company-ID")); company != "" {
				req.Header.Set("X-Company-ID", company)
			}
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

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/rfx-events/", proxyHandler)
	mux.HandleFunc("/api/v1/rfx-events", proxyHandler)
	return listenHTTPServer(t, mux)
}

type browserJWTClaims struct {
	tenantID string
	userID   string
}

func validateBrowserJWT(r *http.Request) (browserJWTClaims, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(auth, "Bearer ") {
		return browserJWTClaims{}, false
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return []byte(browserE2EJWTSecret), nil
	})
	if err != nil || !token.Valid {
		return browserJWTClaims{}, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return browserJWTClaims{}, false
	}
	tenantID, _ := claims["tenant_id"].(string)
	userID, _ := claims["sub"].(string)
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(userID) == "" {
		return browserJWTClaims{}, false
	}
	return browserJWTClaims{tenantID: tenantID, userID: userID}, true
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
	h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
}
