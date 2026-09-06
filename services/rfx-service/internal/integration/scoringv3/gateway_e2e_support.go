//go:build integration

package scoringv3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/http/handlers"
)

const scoringGatewayJWTSecret = "rfx-scoring-gateway-e2e-jwt-secret"

type scoringGatewayProcess struct {
	cmd    *exec.Cmd
	output bytes.Buffer
	done   chan struct{}
}

type scoringIdentityStub struct {
	server      *httptest.Server
	rolesByUser map[string][]string
}

var (
	scoringGatewayBinaryOnce sync.Once
	scoringGatewayBinaryPath string
	scoringGatewayBinaryErr  error
	scoringDownstreamHeadersMu sync.Mutex
	scoringDownstreamHeaders   http.Header
)

func startScoringIdentityStub(t *testing.T, rolesByUser map[string][]string) *scoringIdentityStub {
	t.Helper()
	stub := &scoringIdentityStub{rolesByUser: rolesByUser}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/auth/me") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		roles := stub.rolesByUser[userID]
		if roles == nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"roles": roles})
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *scoringIdentityStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func scoringBuyerJWT(userID, tenantID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"email":     "buyer-scoring-gateway@freight.test",
		"sub":       userID.String(),
		"exp":       time.Now().Add(2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(scoringGatewayJWTSecret))
	return signed
}

func newScoringGatewayRouter(env *testEnv) http.Handler {
	scoreHandler := handlers.NewScoreHandler(env.scoreModelSvc, env.scoringSvc, env.rfxSvc)
	r := chi.NewRouter()
	r.Use(captureScoringDownstreamHeaders)
	r.Route("/v1/rfx-events", func(r chi.Router) {
		r.Get("/{id}/score-model", scoreHandler.GetScoreModel)
		r.Put("/{id}/score-model", scoreHandler.PutScoreModel)
		r.Post("/{id}/score-model/validate", scoreHandler.ValidateScoreModel)
		r.Post("/{id}/score-model/publish", scoreHandler.PublishScoreModel)
	})
	return r
}

func captureScoringDownstreamHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scoringDownstreamHeadersMu.Lock()
		scoringDownstreamHeaders = r.Header.Clone()
		scoringDownstreamHeadersMu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func lastScoringDownstreamHeaders() http.Header {
	scoringDownstreamHeadersMu.Lock()
	defer scoringDownstreamHeadersMu.Unlock()
	if scoringDownstreamHeaders == nil {
		return http.Header{}
	}
	return scoringDownstreamHeaders.Clone()
}

func startScoringHTTPServer(t *testing.T, env *testEnv) (string, *http.Server) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: newScoringGatewayRouter(env), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return "http://" + ln.Addr().String(), srv
}

func startScoringProductionGateway(t *testing.T, rfxServiceURL string, identity *scoringIdentityStub) (string, *scoringGatewayProcess) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	env := []string{
		"AUTH_ENABLED=true",
		"JWT_SECRET=" + scoringGatewayJWTSecret,
		"RFX_SERVICE_URL=" + strings.TrimRight(rfxServiceURL, "/"),
		"IDENTITY_SERVICE_URL=" + identity.URL(),
		"CORS_ALLOWED_ORIGINS=http://127.0.0.1:3020",
		"RATE_LIMIT_ENABLED=false",
		"OPENAPI_DIR=" + filepath.Join(root, "packages", "openapi"),
		"LOG_LEVEL=error",
		"ENVIRONMENT=test",
	}
	gatewayURL, proc := startScoringGatewayProcess(t, env)
	resp, err := http.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("gateway health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gateway health status=%d", resp.StatusCode)
	}
	return gatewayURL, proc
}

func startScoringGatewayProcess(t *testing.T, env []string) (string, *scoringGatewayProcess) {
	t.Helper()
	binaryPath := buildScoringGatewayBinaryOnce(t)
	baseURL, proc, err := startScoringGatewayProcessOnce(binaryPath, env)
	if err != nil {
		t.Fatalf("start gateway: %v\n%s", err, scoringGatewayLogs(proc))
	}
	t.Cleanup(func() { shutdownScoringGatewayProcess(proc) })
	return baseURL, proc
}

func buildScoringGatewayBinaryOnce(t *testing.T) string {
	t.Helper()
	scoringGatewayBinaryOnce.Do(func() {
		root, err := repoRoot()
		if err != nil {
			scoringGatewayBinaryErr = err
			return
		}
		cacheDir := filepath.Join(os.TempDir(), "freight-platform-integration-binaries")
		_ = os.MkdirAll(cacheDir, 0o755)
		binaryName := "api-gateway-scoring-e2e"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		cachedPath := filepath.Join(cacheDir, binaryName)
		build := exec.Command("go", "build", "-o", cachedPath, "./cmd/server")
		build.Dir = filepath.Join(root, "services", "api-gateway")
		if out, err := build.CombinedOutput(); err != nil {
			scoringGatewayBinaryErr = fmt.Errorf("build api-gateway: %w", err)
			_ = out
			return
		}
		scoringGatewayBinaryPath = cachedPath
	})
	if scoringGatewayBinaryErr != nil {
		t.Fatalf("build api-gateway: %v", scoringGatewayBinaryErr)
	}
	return scoringGatewayBinaryPath
}

func startScoringGatewayProcessOnce(binaryPath string, env []string) (string, *scoringGatewayProcess, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cmd := exec.Command(binaryPath)
	cmd.Env = append(append([]string{}, env...), fmt.Sprintf("API_GATEWAY_PORT=%d", port))
	proc := &scoringGatewayProcess{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &proc.output
	cmd.Stderr = &proc.output
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	go func() {
		_, _ = cmd.Process.Wait()
		close(proc.done)
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(120 * time.Second)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		select {
		case <-proc.done:
			return "", proc, fmt.Errorf("gateway exited early")
		default:
		}
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return baseURL, proc, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	shutdownScoringGatewayProcess(proc)
	return "", proc, fmt.Errorf("gateway readiness timeout")
}

func shutdownScoringGatewayProcess(proc *scoringGatewayProcess) {
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return
	}
	_ = proc.cmd.Process.Kill()
	select {
	case <-proc.done:
	case <-time.After(5 * time.Second):
	}
}

func scoringGatewayLogs(proc *scoringGatewayProcess) string {
	if proc == nil {
		return ""
	}
	return proc.output.String()
}

func scoringGatewayRequest(t *testing.T, method, url, token string, companyID uuid.UUID, spoofHeaders map[string]string, body []byte) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if companyID != uuid.Nil {
		req.Header.Set("X-Company-ID", companyID.String())
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range spoofHeaders {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("repo root not found from %s", wd)
}
