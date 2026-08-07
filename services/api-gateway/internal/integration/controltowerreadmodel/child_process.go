//go:build integration

package controltowerreadmodelintegration

import (
	"bytes"
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

type managedProcess struct {
	cmd    *exec.Cmd
	output bytes.Buffer
	done   chan struct{}
}

var (
	binaryBuildMu     sync.Mutex
	binaryBuilders    = map[string]*sync.Once{}
	binaryBuildPaths  = map[string]string{}
	binaryBuildErrors = map[string]error{}
)

func buildServiceBinaryOnce(t *testing.T, serviceRelativeDir, cacheKey string) string {
	t.Helper()
	path, err := buildServiceBinaryOnceCached(serviceRelativeDir, cacheKey)
	if err != nil {
		t.Fatalf("build service binary: %v", err)
	}
	return path
}

func buildServiceBinaryOnceCached(serviceRelativeDir, cacheKey string) (string, error) {
	binaryBuildMu.Lock()
	builder, ok := binaryBuilders[cacheKey]
	if !ok {
		builder = &sync.Once{}
		binaryBuilders[cacheKey] = builder
	}
	binaryBuildMu.Unlock()

	builder.Do(func() {
		repoRoot, locateErr := locateRepoRoot()
		if locateErr != nil {
			binaryBuildMu.Lock()
			binaryBuildErrors[cacheKey] = locateErr
			binaryBuildMu.Unlock()
			return
		}
		cacheDir := filepath.Join(os.TempDir(), "freight-platform-integration-binaries")
		if mkErr := os.MkdirAll(cacheDir, 0o755); mkErr != nil {
			binaryBuildMu.Lock()
			binaryBuildErrors[cacheKey] = mkErr
			binaryBuildMu.Unlock()
			return
		}
		binaryFileName := cacheKey
		if runtime.GOOS == "windows" {
			binaryFileName += ".exe"
		}
		cachedPath := filepath.Join(cacheDir, binaryFileName)
		build := exec.Command("go", "build", "-o", cachedPath, "./cmd/server")
		build.Dir = filepath.Join(repoRoot, serviceRelativeDir)
		if out, buildErr := build.CombinedOutput(); buildErr != nil {
			binaryBuildMu.Lock()
			binaryBuildErrors[cacheKey] = fmt.Errorf("build %s: %w: %s", serviceRelativeDir, buildErr, redactSecrets(string(out)))
			binaryBuildMu.Unlock()
			return
		}
		binaryBuildMu.Lock()
		binaryBuildPaths[cacheKey] = cachedPath
		binaryBuildMu.Unlock()
	})

	binaryBuildMu.Lock()
	buildErr := binaryBuildErrors[cacheKey]
	cachedPath := binaryBuildPaths[cacheKey]
	binaryBuildMu.Unlock()
	if buildErr != nil {
		return "", buildErr
	}
	return cachedPath, nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func startManagedHTTPProcess(t *testing.T, binaryPath string, env []string, portEnvKey, readyPath string) (baseURL string) {
	t.Helper()
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		baseURL, lastErr = startManagedHTTPProcessOnce(t, binaryPath, env, portEnvKey, readyPath)
		if lastErr == nil {
			return baseURL
		}
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
	}
	t.Fatalf("process readiness failed after %d attempts: %v", maxAttempts, lastErr)
	return ""
}

func startManagedHTTPProcessOnce(t *testing.T, binaryPath string, env []string, portEnvKey, readyPath string) (baseURL string, err error) {
	t.Helper()

	port, reserveErr := reserveTCPPort()
	if reserveErr != nil {
		return "", fmt.Errorf("reserve port: %w", reserveErr)
	}

	cmd := exec.Command(binaryPath)
	cmd.Env = append(append([]string{}, env...), fmt.Sprintf("%s=%d", portEnvKey, port))
	proc := &managedProcess{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &proc.output
	cmd.Stderr = &proc.output
	if startErr := cmd.Start(); startErr != nil {
		return "", fmt.Errorf("start process: %w", startErr)
	}
	go func() {
		_, _ = cmd.Process.Wait()
		close(proc.done)
	}()

	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	if waitErr := waitForProcessReady(proc, baseURL+readyPath, 120*time.Second); waitErr != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-proc.done:
		case <-time.After(5 * time.Second):
		}
		return "", fmt.Errorf("%v\nprocess output:\n%s", waitErr, redactSecrets(proc.output.String()))
	}

	t.Cleanup(func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Kill()
		select {
		case <-proc.done:
		case <-time.After(5 * time.Second):
		}
	})
	return baseURL, nil
}

func reserveTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitForProcessReady(proc *managedProcess, endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-proc.done:
			return fmt.Errorf("process exited before ready output=%s", redactSecrets(proc.output.String()))
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
	return fmt.Errorf("timeout waiting for %s output=%s", endpoint, redactSecrets(proc.output.String()))
}

func redactSecrets(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "password=") || strings.Contains(lower, "postgres://") {
		return "[redacted process output]"
	}
	return value
}
