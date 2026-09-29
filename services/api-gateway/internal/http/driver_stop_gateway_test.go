package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/driver"
	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestDriverStopGatewayRoutes(t *testing.T) {
	secret := "driver-stop-test-secret"
	verifiedTenant := uuid.NewString()
	verifiedUser := uuid.NewString()
	spoofTenant := uuid.NewString()
	spoofUser := uuid.NewString()
	var seen struct {
		path, tenant, user, key, driver string
		calls                           int
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.path = r.URL.Path
		seen.tenant = r.Header.Get("X-Tenant-ID")
		seen.user = r.Header.Get("X-User-ID")
		seen.key = r.Header.Get("Idempotency-Key")
		seen.driver = r.Header.Get("X-Driver-ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"CONFLICT","message":"VERSION_CONFLICT"}`))
	}))
	defer backend.Close()
	role := "DRIVER"
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/me" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{role}})
	}))
	defer identity.Close()

	cfg := testConfig()
	cfg.AuthEnabled = true
	cfg.JWTSecret = secret
	cfg.MaxRequestBodyBytes = 1 << 20
	cfg.Services.Shipment = backend.URL
	cfg.Services.Identity = identity.URL
	handler := driver.NewHandler(testLogger(), cfg)
	router := gatewayhttp.NewRouter(testLogger(), cfg, nil, nil, nil, nil, handler)

	stopID := uuid.NewString()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/driver/me/stops/"+stopID+"/arrive", strings.NewReader(`{"expectedVersion":1}`))
	req.Header.Set("Authorization", "Bearer "+signDriverToken(t, secret, verifiedUser, verifiedTenant))
	req.Header.Set("X-Tenant-ID", spoofTenant)
	req.Header.Set("X-User-ID", spoofUser)
	req.Header.Set("X-Driver-ID", uuid.NewString())
	req.Header.Set("Idempotency-Key", "stop-key-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if seen.path != "/v1/driver/me/stops/"+stopID+"/arrive" {
		t.Fatalf("path %s", seen.path)
	}
	if seen.tenant != verifiedTenant || seen.user != verifiedUser {
		t.Fatalf("forwarded tenant %s user %s", seen.tenant, seen.user)
	}
	if seen.key != "stop-key-1" {
		t.Fatalf("idempotency %s", seen.key)
	}
	if seen.driver != "" {
		t.Fatalf("driver header forwarded %s", seen.driver)
	}

	before := seen.calls
	role = "CARRIER_DISPATCHER"
	denied := httptest.NewRequest(http.MethodGet, "/api/v1/driver/me/stops", nil)
	denied.Header.Set("Authorization", "Bearer "+signDriverToken(t, secret, verifiedUser, verifiedTenant))
	deniedRec := httptest.NewRecorder()
	router.ServeHTTP(deniedRec, denied)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("dispatcher status %d body %s", deniedRec.Code, deniedRec.Body.String())
	}
	if seen.calls != before {
		t.Fatal("non-driver reached shipment-service")
	}

	missing := httptest.NewRequest(http.MethodPost, "/api/v1/driver/me/stops/"+stopID+"/complete", strings.NewReader(`{}`))
	missingRec := httptest.NewRecorder()
	router.ServeHTTP(missingRec, missing)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth %d", missingRec.Code)
	}
}

func signDriverToken(t *testing.T, secret, userID, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"sub":       userID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
