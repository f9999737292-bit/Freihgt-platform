//go:build integration

package studio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type browserGatewayProcess struct {
	cmd    *exec.Cmd
	output bytes.Buffer
	done   chan struct{}
}

var (
	apiGatewayBinaryMu     sync.Mutex
	apiGatewayBinaryOnce   sync.Once
	apiGatewayBinaryPath   string
	apiGatewayBinaryBuildErr error
)

func buildAPIGatewayBinaryOnce(t *testing.T) string {
	t.Helper()
	apiGatewayBinaryOnce.Do(func() {
		root, err := repoRoot()
		if err != nil {
			apiGatewayBinaryBuildErr = err
			return
		}
		cacheDir := filepath.Join(os.TempDir(), "freight-platform-integration-binaries")
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			apiGatewayBinaryBuildErr = err
			return
		}
		binaryName := "api-gateway-browser-e2e"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		cachedPath := filepath.Join(cacheDir, binaryName)
		build := exec.Command("go", "build", "-o", cachedPath, "./cmd/server")
		build.Dir = filepath.Join(root, "services", "api-gateway")
		if out, err := build.CombinedOutput(); err != nil {
			apiGatewayBinaryBuildErr = fmt.Errorf("build api-gateway: %w: %s", err, redactProcessOutput(string(out)))
			return
		}
		apiGatewayBinaryPath = cachedPath
	})
	if apiGatewayBinaryBuildErr != nil {
		t.Fatalf("build api-gateway binary: %v", apiGatewayBinaryBuildErr)
	}
	return apiGatewayBinaryPath
}

func startProductionGatewayProcess(t *testing.T, env []string) (baseURL string, proc *browserGatewayProcess) {
	t.Helper()
	binaryPath := buildAPIGatewayBinaryOnce(t)

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		baseURL, proc, lastErr = startProductionGatewayProcessOnce(binaryPath, env)
		if lastErr == nil {
			t.Cleanup(func() { shutdownBrowserGatewayProcess(proc) })
			return baseURL, proc
		}
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
	}
	t.Fatalf("production api-gateway readiness failed after %d attempts: %v\n%s", maxAttempts, lastErr, procLogs(proc))
	return "", nil
}

func startProductionGatewayProcessOnce(binaryPath string, env []string) (string, *browserGatewayProcess, error) {
	port, err := reserveLocalTCPPort()
	if err != nil {
		return "", nil, fmt.Errorf("reserve port: %w", err)
	}

	cmd := exec.Command(binaryPath)
	cmd.Env = append(append([]string{}, env...), fmt.Sprintf("API_GATEWAY_PORT=%d", port))
	proc := &browserGatewayProcess{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &proc.output
	cmd.Stderr = &proc.output
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start api-gateway: %w", err)
	}
	go func() {
		_, _ = cmd.Process.Wait()
		close(proc.done)
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForGatewayHealth(proc, baseURL+"/health", 120*time.Second); err != nil {
		shutdownBrowserGatewayProcess(proc)
		return "", proc, err
	}
	return baseURL, proc, nil
}

func waitForGatewayHealth(proc *browserGatewayProcess, endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		select {
		case <-proc.done:
			return fmt.Errorf("api-gateway exited before ready: %s", procLogs(proc))
		default:
		}
		resp, err := client.Get(endpoint)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s: %s", endpoint, procLogs(proc))
}

func shutdownBrowserGatewayProcess(proc *browserGatewayProcess) {
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return
	}
	_ = proc.cmd.Process.Kill()
	select {
	case <-proc.done:
	case <-time.After(5 * time.Second):
	}
}

func procLogs(proc *browserGatewayProcess) string {
	if proc == nil {
		return ""
	}
	return redactProcessOutput(proc.output.String())
}

func reserveLocalTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func redactProcessOutput(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "password=") || strings.Contains(lower, "postgres://") || strings.Contains(lower, "jwt_secret=") {
		return "[redacted process output]"
	}
	return value
}

func dumpGatewayLogsOnFailure(t *testing.T, proc *browserGatewayProcess) {
	t.Helper()
	if proc == nil || !t.Failed() {
		return
	}
	logs := procLogs(proc)
	if logs == "" {
		return
	}
	t.Logf("production api-gateway logs:\n%s", logs)
}

func writeGatewayFailureArtifact(t *testing.T, proc *browserGatewayProcess) {
	t.Helper()
	if proc == nil || !t.Failed() {
		return
	}
	root, err := repoRoot()
	if err != nil {
		return
	}
	dir := filepath.Join(root, "apps", "web-procurement", "e2e", "rfx-studio", "test-results")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "api-gateway.log")
	_ = os.WriteFile(path, []byte(procLogs(proc)), 0o644)
}

func readGatewayRoutes(t *testing.T, gatewayURL string) []map[string]string {
	t.Helper()
	resp, err := http.Get(gatewayURL + "/routes")
	if err != nil {
		t.Fatalf("get gateway routes: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read gateway routes: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gateway routes status=%d body=%s", resp.StatusCode, string(body))
	}
	var payload struct {
		Routes []map[string]string `json:"routes"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode gateway routes: %v", err)
	}
	return payload.Routes
}
