package http_test

import (
	"encoding/json"
	"io"
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

func TestDriverDeliveryDispositionGatewayRoute(t *testing.T) {
	secret := "driver-disposition-test-secret"
	verifiedTenant := uuid.NewString()
	verifiedUser := uuid.NewString()
	stopID := uuid.NewString()
	actionID := uuid.NewString()
	body := `{"shipmentId":"` + uuid.NewString() + `","cargoId":"` + uuid.NewString() + `","acceptedQuantity":2,"rejectedQuantity":1,"uom":"PALLET","reasonCode":"DAMAGE","reasonComment":"torn wrap","occurredAt":"2026-10-06T12:00:00Z","evidence":[]}`
	var seen struct {
		path, tenant, user, key, driver, raw string
		status                               int
		calls                                int
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.path = r.URL.Path
		seen.tenant = r.Header.Get("X-Tenant-ID")
		seen.user = r.Header.Get("X-User-ID")
		seen.key = r.Header.Get("Idempotency-Key")
		seen.driver = r.Header.Get("X-Driver-ID")
		raw, _ := io.ReadAll(r.Body)
		seen.raw = string(raw)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(seen.status)
		_, _ = w.Write([]byte(`{"code":"UPSTREAM","message":"kept"}`))
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
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler := driver.NewHandler(testLogger(), cfg)
	router := gatewayhttp.NewRouter(testLogger(), cfg, proxy, nil, nil, nil, handler)
	path := "/api/v1/driver/me/stops/" + stopID + "/actions/" + actionID + "/delivery-disposition"

	seen.status = http.StatusBadRequest
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signDriverToken(t, secret, verifiedUser, verifiedTenant))
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Driver-ID", uuid.NewString())
	req.Header.Set("Idempotency-Key", "disposition-key-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upstream 400 became %d body %s", rec.Code, rec.Body.String())
	}
	if seen.path != "/v1/driver/me/stops/"+stopID+"/actions/"+actionID+"/delivery-disposition" {
		t.Fatalf("path %s", seen.path)
	}
	if seen.tenant != verifiedTenant || seen.user != verifiedUser {
		t.Fatalf("forwarded tenant %s user %s", seen.tenant, seen.user)
	}
	if seen.key != "disposition-key-1" {
		t.Fatalf("idempotency %s", seen.key)
	}
	if seen.driver != "" {
		t.Fatalf("driver header forwarded %s", seen.driver)
	}
	if seen.raw != body {
		t.Fatalf("body changed %s", seen.raw)
	}

	seen.status = http.StatusInternalServerError
	upstream := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	upstream.Header.Set("Authorization", "Bearer "+signDriverToken(t, secret, verifiedUser, verifiedTenant))
	upstream.Header.Set("Idempotency-Key", "disposition-key-1")
	upstreamRec := httptest.NewRecorder()
	router.ServeHTTP(upstreamRec, upstream)
	if upstreamRec.Code != http.StatusInternalServerError {
		t.Fatalf("upstream 500 became %d", upstreamRec.Code)
	}

	before := seen.calls
	missing := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	missingRec := httptest.NewRecorder()
	router.ServeHTTP(missingRec, missing)
	if missingRec.Code != http.StatusUnauthorized || seen.calls != before {
		t.Fatalf("missing auth %d calls %d", missingRec.Code, seen.calls)
	}
	invalid := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	invalid.Header.Set("Authorization", "Bearer not-a-token")
	invalidRec := httptest.NewRecorder()
	router.ServeHTTP(invalidRec, invalid)
	if invalidRec.Code != http.StatusUnauthorized || seen.calls != before {
		t.Fatalf("invalid auth %d calls %d", invalidRec.Code, seen.calls)
	}

	role = "CARRIER_DISPATCHER"
	denied := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	denied.Header.Set("Authorization", "Bearer "+signDriverToken(t, secret, verifiedUser, verifiedTenant))
	denied.Header.Set("Idempotency-Key", "disposition-key-1")
	deniedRec := httptest.NewRecorder()
	router.ServeHTTP(deniedRec, denied)
	if deniedRec.Code != http.StatusForbidden || seen.calls != before {
		t.Fatalf("dispatcher %d calls %d", deniedRec.Code, seen.calls)
	}

	for _, blocked := range []string{
		"/api/v1/driver/me/stops/" + stopID + "/authorize-return",
		"/api/v1/driver/me/stops/" + stopID + "/actions/" + actionID + "/authorize-redirect",
		"/api/v1/driver/me/delivery-dispositions/" + uuid.NewString() + "/authorize-return",
		"/api/v1/driver/me/delivery-dispositions/" + uuid.NewString() + "/authorize-redirect",
		"/api/v1/driver/me/delivery-dispositions/" + uuid.NewString() + "/hold",
	} {
		blockedReq := httptest.NewRequest(http.MethodPost, blocked, strings.NewReader(`{}`))
		blockedReq.Header.Set("Authorization", "Bearer "+signDriverToken(t, secret, verifiedUser, verifiedTenant))
		blockedRec := httptest.NewRecorder()
		router.ServeHTTP(blockedRec, blockedReq)
		if blockedRec.Code != http.StatusNotFound || seen.calls != before {
			t.Fatalf("route %s status %d calls %d", blocked, blockedRec.Code, seen.calls)
		}
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
