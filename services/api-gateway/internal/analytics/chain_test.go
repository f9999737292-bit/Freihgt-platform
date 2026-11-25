package analytics_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestOperationsFoundationChain(t *testing.T) {
	const (
		secret       = "chain-secret"
		sharedToken  = "shared-internal-token"
		gatewayToken = "gateway-caller-token"
		sourceToken  = "source-caller-token"
		tenantA      = "11111111-1111-1111-1111-111111111111"
		tenantB      = "22222222-2222-2222-2222-222222222222"
	)
	var sourceTenant, sourceCaller, presentedSourceToken, sourceScope, sourceCompany string
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			w.WriteHeader(http.StatusOK)
		case "/internal/v1/analytics/operations-foundation-v2":
			sourceTenant = r.Header.Get("X-Tenant-ID")
			sourceCaller = r.Header.Get("X-Internal-Service-Name")
			presentedSourceToken = r.Header.Get("X-Internal-Service-Token")
			sourceScope = r.Header.Get("X-Analytics-Scope")
			sourceCompany = r.Header.Get("X-Analytics-Company-ID")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenantId":                  tenantA,
				"shipmentTotal":             10,
				"onTimePickupDenominator":   4,
				"onTimePickupNumerator":     2,
				"onTimeDeliveryDenominator": 10,
				"onTimeDeliveryNumerator":   8,
				"returnCaseCount":           2,
				"redirectCaseCount":         1,
				"carriers": []any{
					map[string]any{"carrierCompanyId": "33333333-3333-3333-3333-333333333333", "onTimePickupDenominator": 1, "onTimePickupNumerator": 1, "onTimeDeliveryDenominator": 2, "onTimeDeliveryNumerator": 1},
					map[string]any{"carrierCompanyId": "22222222-2222-2222-2222-222222222222", "onTimePickupDenominator": 1, "onTimePickupNumerator": 0, "onTimeDeliveryDenominator": 2, "onTimeDeliveryNumerator": 2},
				},
				"dispositionScope": "TENANT",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer shipment.Close()

	analyticsURL, logs, stop := startAnalyticsProcess(t, shipment.URL, gatewayToken, sourceToken, sharedToken)
	defer stop()

	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/roles"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"code": "PLATFORM_ADMIN"}}})
		case strings.Contains(r.URL.Path, "/companies"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer identity.Close()

	cfg := config.Config{
		AuthEnabled:                 true,
		JWTSecret:                   secret,
		ProxyTimeoutSeconds:         5,
		MaxRequestBodyBytes:         1 << 20,
		InternalServiceToken:        sharedToken,
		AnalyticsGatewayCallerToken: gatewayToken,
		Services: config.ServiceURLs{
			Identity:  identity.URL,
			Analytics: analyticsURL,
		},
	}
	proxy, err := gatewayhttp.NewProxyHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	gateway := gatewayhttp.NewRouter(log, cfg, proxy, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/kpis/OPS_ON_TIME_DELIVERY_RATE", nil)
	req.Header.Set("Authorization", "Bearer "+signChainToken(t, secret, tenantA))
	req.Header.Set("X-Tenant-ID", tenantB)
	req.Header.Set("X-User-ID", "spoof-user")
	req.Header.Set("X-Internal-Service-Token", "spoof-token")
	req.Header.Set("X-Internal-Service-Name", "spoof-service")
	rec := httptest.NewRecorder()
	gateway.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		KPIID             string `json:"kpiId"`
		DefinitionVersion int    `json:"definitionVersion"`
		Measure           struct {
			Type        string   `json:"type"`
			Value       *float64 `json:"value"`
			Numerator   *int64   `json:"numerator"`
			Denominator *int64   `json:"denominator"`
		} `json:"measure"`
		DataFreshness struct {
			Status string `json:"status"`
		} `json:"dataFreshness"`
		Completeness string `json:"completeness"`
		GeneratedAt  string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.KPIID != "OPS_ON_TIME_DELIVERY_RATE" || payload.DefinitionVersion != 1 || payload.Measure.Type != "RATIO" || payload.Measure.Value == nil || *payload.Measure.Value != 0.8 || payload.Measure.Numerator == nil || *payload.Measure.Numerator != 8 || payload.Measure.Denominator == nil || *payload.Measure.Denominator != 10 {
		t.Fatalf("mapping %+v", payload)
	}
	if payload.Completeness != "PARTIAL" || payload.DataFreshness.Status != "UNKNOWN" || payload.GeneratedAt == "" {
		t.Fatalf("quality %+v", payload)
	}
	if sourceTenant != tenantA || sourceCaller != "analytics-service" || presentedSourceToken != sourceToken || sourceScope != "TENANT" || sourceCompany != "" {
		t.Fatalf("source tenant=%s caller=%s scope=%s company=%s dedicated=%t", sourceTenant, sourceCaller, sourceScope, sourceCompany, presentedSourceToken == sourceToken)
	}

	carrierReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/kpis/CAR_ON_TIME_PICKUP_RATE", nil)
	carrierReq.Header.Set("Authorization", "Bearer "+signChainToken(t, secret, tenantA))
	carrierReq.Header.Set("X-Tenant-ID", tenantB)
	carrierRec := httptest.NewRecorder()
	gateway.ServeHTTP(carrierRec, carrierReq)
	if carrierRec.Code != http.StatusOK {
		t.Fatalf("carrier status=%d body=%s", carrierRec.Code, carrierRec.Body.String())
	}
	var carrier struct {
		KPIID     string `json:"kpiId"`
		Dimension string `json:"dimension"`
		Items     []struct {
			CarrierCompanyID string `json:"carrierCompanyId"`
			Measure          struct {
				Value       *float64 `json:"value"`
				Numerator   *int64   `json:"numerator"`
				Denominator *int64   `json:"denominator"`
			} `json:"measure"`
		} `json:"items"`
		Completeness string `json:"completeness"`
	}
	if err := json.Unmarshal(carrierRec.Body.Bytes(), &carrier); err != nil {
		t.Fatal(err)
	}
	if carrier.KPIID != "CAR_ON_TIME_PICKUP_RATE" || carrier.Dimension != "CARRIER" || carrier.Completeness != "PARTIAL" || len(carrier.Items) != 2 {
		t.Fatalf("%+v", carrier)
	}
	if carrier.Items[0].CarrierCompanyID != "22222222-2222-2222-2222-222222222222" || carrier.Items[0].Measure.Value == nil || *carrier.Items[0].Measure.Value != 0 || carrier.Items[1].CarrierCompanyID != "33333333-3333-3333-3333-333333333333" || carrier.Items[1].Measure.Value == nil || *carrier.Items[1].Measure.Value != 1 {
		t.Fatalf("%+v", carrier.Items)
	}
	if strings.Contains(carrierRec.Body.String(), `"shipmentTotal"`) {
		t.Fatalf("tenant aggregate leaked: %s", carrierRec.Body.String())
	}

	querySpoof, err := http.NewRequest(http.MethodGet, analyticsURL+"/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL?tenant_id="+tenantB, nil)
	if err != nil {
		t.Fatal(err)
	}
	querySpoof.Header.Set("X-Internal-Service-Token", gatewayToken)
	querySpoof.Header.Set("X-Internal-Service-Name", "api-gateway")
	querySpoof.Header.Set("X-Tenant-ID", tenantA)
	querySpoof.Header.Set("X-Analytics-Scope", "TENANT")
	queryResp, err := http.DefaultClient.Do(querySpoof)
	if err != nil {
		t.Fatal(err)
	}
	queryBody, _ := io.ReadAll(queryResp.Body)
	_ = queryResp.Body.Close()
	if queryResp.StatusCode != http.StatusBadRequest || bytes.Contains(queryBody, []byte(`"value"`)) {
		t.Fatalf("query tenant status=%d body=%s", queryResp.StatusCode, queryBody)
	}
	wrongName, err := http.NewRequest(http.MethodGet, analyticsURL+"/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", nil)
	if err != nil {
		t.Fatal(err)
	}
	wrongName.Header.Set("X-Internal-Service-Token", gatewayToken)
	wrongName.Header.Set("X-Internal-Service-Name", "analytics-service")
	wrongName.Header.Set("X-Tenant-ID", tenantA)
	wrongName.Header.Set("X-Analytics-Scope", "TENANT")
	wrongNameResp, err := http.DefaultClient.Do(wrongName)
	if err != nil {
		t.Fatal(err)
	}
	wrongNameBody, _ := io.ReadAll(wrongNameResp.Body)
	_ = wrongNameResp.Body.Close()
	if wrongNameResp.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong name status=%d body=%s", wrongNameResp.StatusCode, wrongNameBody)
	}

	spoof, err := http.NewRequest(http.MethodGet, analyticsURL+"/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL?tenant_id="+tenantB, nil)
	if err != nil {
		t.Fatal(err)
	}
	spoof.Header.Set("X-Internal-Service-Token", sharedToken)
	spoof.Header.Set("X-Internal-Service-Name", "api-gateway")
	spoof.Header.Set("X-Tenant-ID", tenantB)
	spoof.Header.Set("X-Analytics-Scope", "TENANT")
	spoofResp, err := http.DefaultClient.Do(spoof)
	if err != nil {
		t.Fatal(err)
	}
	spoofBody, _ := io.ReadAll(spoofResp.Body)
	_ = spoofResp.Body.Close()
	if spoofResp.StatusCode != http.StatusUnauthorized && spoofResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("shared token status=%d body=%s", spoofResp.StatusCode, spoofBody)
	}
	foreign, err := http.NewRequest(http.MethodGet, analyticsURL+"/v1/analytics/kpis/OPS_SHIPMENTS_TOTAL", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign.Header.Set("X-Internal-Service-Token", sourceToken)
	foreign.Header.Set("X-Internal-Service-Name", "api-gateway")
	foreign.Header.Set("X-Tenant-ID", tenantA)
	foreign.Header.Set("X-Analytics-Scope", "TENANT")
	foreignResp, err := http.DefaultClient.Do(foreign)
	if err != nil {
		t.Fatal(err)
	}
	foreignBody, _ := io.ReadAll(foreignResp.Body)
	_ = foreignResp.Body.Close()
	if foreignResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("foreign secret status=%d body=%s", foreignResp.StatusCode, foreignBody)
	}
	for _, secretValue := range []string{sharedToken, gatewayToken, sourceToken} {
		if bytes.Contains(logs.Bytes(), []byte(secretValue)) || bytes.Contains(spoofBody, []byte(secretValue)) || bytes.Contains(foreignBody, []byte(secretValue)) || bytes.Contains(queryBody, []byte(secretValue)) || bytes.Contains(wrongNameBody, []byte(secretValue)) {
			t.Fatal("credential appeared in analytics logs or an error response")
		}
	}
}

func startAnalyticsProcess(t *testing.T, shipmentURL, gatewayToken, sourceToken, sharedToken string) (string, *bytes.Buffer, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	serviceDir, err := filepath.Abs(filepath.Join("..", "..", "..", "analytics-service"))
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "analytics-service.exe")
	build := exec.Command("go", "build", "-o", exe, "./cmd/server")
	build.Dir = serviceDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build analytics-service: %v\n%s", err, out)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		"ANALYTICS_SERVICE_PORT="+strconv.Itoa(port),
		"SHIPMENT_SERVICE_URL="+shipmentURL,
		"ANALYTICS_GATEWAY_CALLER_TOKEN="+gatewayToken,
		"ANALYTICS_SOURCE_CALLER_TOKEN="+sourceToken,
		"INTERNAL_SERVICE_TOKEN="+sharedToken,
		"ENVIRONMENT=test",
		"LOG_LEVEL=error",
	)
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	stop := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return base, &logs, stop
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	t.Fatalf("analytics-service did not become healthy\n%s", logs.String())
	return "", &logs, func() {}
}

func signChainToken(t *testing.T, secret, tenantID string) string {
	t.Helper()
	claims := jwt.MapClaims{"tenant_id": tenantID, "sub": "user-1", "exp": time.Now().Add(time.Hour).Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
