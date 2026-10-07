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
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	gatewayhttp "github.com/freight-platform/api-gateway/internal/http"
)

func TestOperationsFoundationChain(t *testing.T) {
	const (
		secret      = "chain-secret"
		sharedToken = "shared-internal-token"
		tenantA     = "11111111-1111-1111-1111-111111111111"
		tenantB     = "22222222-2222-2222-2222-222222222222"
	)
	var sourceTenant, sourceCaller, sourceToken string
	shipment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			w.WriteHeader(http.StatusOK)
		case "/internal/v1/analytics/operations-foundation":
			sourceTenant = r.Header.Get("X-Tenant-ID")
			sourceCaller = r.Header.Get("X-Internal-Service-Name")
			sourceToken = r.Header.Get("X-Internal-Service-Token")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenantId":                  tenantA,
				"shipmentTotal":             10,
				"onTimeDeliveryDenominator": 10,
				"onTimeDeliveryNumerator":   8,
				"returnCaseCount":           2,
				"redirectCaseCount":         1,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer shipment.Close()

	analyticsURL, stop := startAnalyticsProcess(t, shipment.URL, sharedToken)
	defer stop()

	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": []string{"CARRIER_DISPATCHER"}})
	}))
	defer identity.Close()

	cfg := config.Config{
		AuthEnabled:          true,
		JWTSecret:            secret,
		ProxyTimeoutSeconds:  5,
		MaxRequestBodyBytes:  1 << 20,
		InternalServiceToken: sharedToken,
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
	if sourceTenant != tenantA || sourceCaller != "analytics-service" || sourceToken != sharedToken {
		t.Fatalf("source tenant=%s caller=%s tokenSet=%t", sourceTenant, sourceCaller, sourceToken == sharedToken)
	}
}

func startAnalyticsProcess(t *testing.T, shipmentURL, token string) (string, func()) {
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
		"INTERNAL_SERVICE_TOKEN="+token,
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
				return base, stop
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	t.Fatalf("analytics-service did not become healthy\n%s", logs.String())
	return "", func() {}
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
