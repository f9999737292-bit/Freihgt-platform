//go:build integration

package studio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

const nuxtShutdownWait = 10 * time.Second

// stopNuxtDevProcess terminates the launcher command tree and any task-owned
// listeners/wrappers still bound to port, then verifies the port is free.
func stopNuxtDevProcess(t *testing.T, cmd *exec.Cmd, port string) {
	t.Helper()
	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		if err := killProcessTreePID(pid); err != nil {
			t.Logf("kill nuxt launcher tree pid=%d: %v", pid, err)
		}
		if !waitForProcessExitBounded(pid, nuxtShutdownWait) {
			t.Logf("nuxt launcher pid=%d still running after bounded wait", pid)
		}
		_ = cmd.Wait()
	}
	if port == "" || !isScopedDevPort(port) {
		return
	}
	worktree, err := repoRoot()
	if err != nil {
		t.Logf("repo root for nuxt port cleanup: %v", err)
		worktree = ""
	}
	releaseTaskOwnedDevPort(t, port, worktree)
}

func releaseTaskOwnedDevPort(t *testing.T, port, worktreeRoot string) {
	t.Helper()
	if port == "" || !isScopedDevPort(port) {
		return
	}
	ensureDevPortFree(t, port)
	if devPortInUse(port) {
		t.Logf("dev port %s still in use after task-owned cleanup", port)
	}
}

func verifyDevPortsReleased(t *testing.T, ports ...string) {
	t.Helper()
	for _, port := range ports {
		if !isScopedDevPort(port) {
			continue
		}
		waitUntilDevPortFree(t, port, nuxtShutdownWait)
	}
}

func startNuxtDevCommand(t *testing.T, ctx context.Context, appLabel, port string, env []string) (*exec.Cmd, *os.File) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	filter, err := nuxtPackageFilter(appLabel)
	if err != nil {
		t.Fatalf("nuxt package filter: %v", err)
	}
	logFile, err := os.CreateTemp("", fmt.Sprintf("rfx-scoring-%s-%s-*.log", appLabel, port))
	if err != nil {
		t.Fatalf("create nuxt log: %v", err)
	}
	if err := runNuxtPrepare(ctx, root, filter, env); err != nil {
		_ = logFile.Close()
		t.Fatalf("prepare %s before dev on port %s: %v", appLabel, port, err)
	}
	cmd := newNuxtDevCommand(ctx, root, filter, port, env)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start %s dev on port %s: %v", appLabel, port, err)
	}
	return cmd, logFile
}
