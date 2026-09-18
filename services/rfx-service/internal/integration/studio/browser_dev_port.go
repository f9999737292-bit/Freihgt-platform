//go:build integration

package studio

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func ensureDevPortFree(t *testing.T, port string) {
	t.Helper()
	if !isScopedDevPort(port) {
		t.Fatalf("dev port %s is out of cleanup scope", port)
	}
	worktree, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root for port cleanup: %v", err)
	}
	if runtime.GOOS == "windows" {
		ensureDevPortFreeWindows(t, port, worktree)
		return
	}
	ensureDevPortFreeUnix(t, port, worktree)
}

func ensureDevPortFreeWindows(t *testing.T, port, worktreeRoot string) {
	t.Helper()
	listeners := listeningPIDsWindows(t, port)
	for pid := range listeners {
		proc, ok := lookupWindowsProcess(t, pid)
		if !ok {
			if !pidStillListeningWindows(port, pid) {
				continue
			}
			t.Fatalf("UNKNOWN_PORT_OWNER_BLOCKED=YES dev port %s pid=%d process lookup failed", port, pid)
		}
		kill, blocked, reason := classifyDevPortListener(port, proc, worktreeRoot)
		if blocked {
			t.Fatalf(
				"UNKNOWN_PORT_OWNER_BLOCKED=YES dev port %s pid=%d process=%q cmd=%q reason=%s",
				port,
				proc.PID,
				proc.ProcessName,
				sanitizeCommandLineForLog(proc.CommandLine, 160),
				reason,
			)
		}
		if kill {
			t.Logf("freeing task-owned stale nuxt dev on %s (pid=%d cmd=%q)", port, proc.PID, sanitizeCommandLineForLog(proc.CommandLine, 120))
			if killErr := killProcessTreePID(proc.PID); killErr != nil {
				t.Fatalf("kill task-owned stale nuxt pid=%d on port %s: %v", proc.PID, port, killErr)
			}
			if !waitForProcessExitBounded(proc.PID, 10*time.Second) {
				t.Fatalf("task-owned stale nuxt pid=%d on port %s did not exit before restart", proc.PID, port)
			}
		}
	}
	killVerifiedStaleNuxtDevWindows(t, port, worktreeRoot)
	killVerifiedStalePnpmNuxtLaunchersWindows(t, port, worktreeRoot)
	waitUntilDevPortFree(t, port, 10*time.Second)
}

func killVerifiedStalePnpmNuxtLaunchersWindows(t *testing.T, port, worktreeRoot string) {
	t.Helper()
	script := fmt.Sprintf(
		`Get-CimInstance Win32_Process -Filter "Name='node.exe'" | Where-Object { $_.CommandLine -match 'pnpm' -and $_.CommandLine -match 'exec' -and $_.CommandLine -match 'nuxt' -and $_.CommandLine -match '--port' -and $_.CommandLine -match '%s' } | ForEach-Object { $cmd=$_.CommandLine; if ($null -eq $cmd) { $cmd="" }; Write-Output $_.ProcessId; Write-Output $_.Name; Write-Output $cmd; Write-Output "---" }`,
		port,
	)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil && len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	for _, chunk := range strings.Split(strings.TrimSpace(string(out)), "---") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		lines := strings.Split(chunk, "\n")
		if len(lines) < 3 {
			continue
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(lines[0]))
		if convErr != nil || pid <= 0 {
			continue
		}
		proc := portProcessInfo{
			PID:         pid,
			ProcessName: strings.TrimSpace(lines[1]),
			CommandLine: strings.TrimSpace(strings.Join(lines[2:], "\n")),
		}
		if !isTaskOwnedStalePnpmNuxtLauncher(port, proc.ProcessName, proc.CommandLine, worktreeRoot) {
			continue
		}
		t.Logf("freeing verified stale pnpm nuxt launcher on %s (pid=%d)", port, proc.PID)
		_ = killProcessTreePID(proc.PID)
		_ = waitForProcessExitBounded(proc.PID, 10*time.Second)
	}
}

func ensureDevPortFreeUnix(t *testing.T, port, worktreeRoot string) {
	t.Helper()
	out, err := exec.Command("lsof", "-ti", "tcp:"+port).CombinedOutput()
	if err != nil {
		if len(strings.TrimSpace(string(out))) == 0 {
			waitUntilDevPortFree(t, port, 10*time.Second)
			return
		}
		t.Fatalf("inspect dev port %s: %v", port, err)
	}
	for _, pidStr := range strings.Fields(string(out)) {
		pid, convErr := strconv.Atoi(pidStr)
		if convErr != nil || pid <= 0 {
			t.Fatalf("UNKNOWN_PORT_OWNER_BLOCKED=YES dev port %s pid=%q invalid", port, pidStr)
		}
		proc, ok := lookupUnixProcess(t, pid)
		if !ok {
			t.Fatalf("UNKNOWN_PORT_OWNER_BLOCKED=YES dev port %s pid=%d process lookup failed", port, pid)
		}
		kill, blocked, reason := classifyDevPortListener(port, proc, worktreeRoot)
		if blocked {
			t.Fatalf(
				"UNKNOWN_PORT_OWNER_BLOCKED=YES dev port %s pid=%d process=%q cmd=%q reason=%s",
				port,
				proc.PID,
				proc.ProcessName,
				sanitizeCommandLineForLog(proc.CommandLine, 160),
				reason,
			)
		}
		if kill {
			t.Logf("freeing task-owned stale nuxt dev on %s (pid=%d cmd=%q)", port, proc.PID, sanitizeCommandLineForLog(proc.CommandLine, 120))
			if killErr := exec.Command("kill", "-9", pidStr).Run(); killErr != nil {
				t.Fatalf("kill task-owned stale nuxt pid=%d on port %s: %v", pid, port, killErr)
			}
		}
	}
	waitUntilDevPortFree(t, port, 10*time.Second)
}

func pidStillListeningWindows(port string, pid int) bool {
	out, err := exec.Command("cmd", "/c", "netstat -ano -p tcp").CombinedOutput()
	if err != nil {
		return true
	}
	needle := ":" + port
	pidNeedle := fmt.Sprintf(" %d", pid)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(strings.ToUpper(line), "LISTENING") &&
			strings.Contains(line, needle) &&
			strings.HasSuffix(strings.TrimSpace(line), pidNeedle) {
			return true
		}
	}
	return false
}

func listeningPIDsWindows(t *testing.T, port string) map[int]struct{} {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "netstat -ano -p tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("inspect dev port %s: %v", port, err)
	}
	needle := ":" + port
	seen := map[int]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(strings.ToUpper(line), "LISTENING") || !strings.Contains(line, needle) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pidStr := fields[len(fields)-1]
		pid, convErr := strconv.Atoi(pidStr)
		if convErr != nil || pid <= 0 {
			continue
		}
		seen[pid] = struct{}{}
	}
	return seen
}

func lookupWindowsProcess(t *testing.T, pid int) (portProcessInfo, bool) {
	t.Helper()
	script := fmt.Sprintf(
		`$p=Get-CimInstance Win32_Process -Filter "ProcessId=%d"; if ($null -eq $p) { exit 2 }; $cmd=$p.CommandLine; if ($null -eq $cmd) { $cmd="" }; Write-Output $p.Name; Write-Output $cmd`,
		pid,
	)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil {
		return portProcessInfo{}, false
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return portProcessInfo{}, false
	}
	name := strings.TrimSpace(parts[0])
	cmdLine := ""
	if len(parts) > 1 {
		cmdLine = strings.TrimSpace(parts[1])
	}
	cwd := lookupProcessCwd(pid)
	return portProcessInfo{PID: pid, ProcessName: name, CommandLine: cmdLine, Cwd: cwd}, true
}

func lookupUnixProcess(t *testing.T, pid int) (portProcessInfo, bool) {
	t.Helper()
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=", "-o", "args=").CombinedOutput()
	if err != nil {
		return portProcessInfo{}, false
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return portProcessInfo{}, false
	}
	fields := strings.Fields(line)
	name := fields[0]
	cmdLine := line
	if len(fields) > 1 {
		cmdLine = strings.TrimSpace(strings.TrimPrefix(line, name))
	}
	cwd := lookupProcessCwd(pid)
	return portProcessInfo{PID: pid, ProcessName: name, CommandLine: cmdLine, Cwd: cwd}, true
}

func killVerifiedStaleNuxtDevWindows(t *testing.T, port, worktreeRoot string) {
	t.Helper()
	script := fmt.Sprintf(
		`Get-CimInstance Win32_Process -Filter "Name='node.exe'" | Where-Object { $_.CommandLine -match 'nuxt' -and $_.CommandLine -match 'dev' -and $_.CommandLine -match '--port' -and $_.CommandLine -match '%s' } | ForEach-Object { $cmd=$_.CommandLine; if ($null -eq $cmd) { $cmd="" }; Write-Output $_.ProcessId; Write-Output $_.Name; Write-Output $cmd; Write-Output "---" }`,
		port,
	)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil && len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	for _, chunk := range strings.Split(strings.TrimSpace(string(out)), "---") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		lines := strings.Split(chunk, "\n")
		if len(lines) < 3 {
			continue
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(lines[0]))
		if convErr != nil || pid <= 0 {
			continue
		}
		proc := portProcessInfo{
			PID:         pid,
			ProcessName: strings.TrimSpace(lines[1]),
			CommandLine: strings.TrimSpace(strings.Join(lines[2:], "\n")),
		}
		kill, blocked, reason := classifyDevPortListener(port, proc, worktreeRoot)
		if blocked {
			t.Fatalf(
				"UNKNOWN_PORT_OWNER_BLOCKED=YES stale nuxt scan port %s pid=%d process=%q cmd=%q reason=%s",
				port,
				proc.PID,
				proc.ProcessName,
				sanitizeCommandLineForLog(proc.CommandLine, 160),
				reason,
			)
		}
		if kill {
			t.Logf("freeing verified stale nuxt dev on %s (pid=%d)", port, proc.PID)
			_ = killProcessTreePID(proc.PID)
			_ = waitForProcessExitBounded(proc.PID, 10*time.Second)
		}
	}
}

func waitUntilDevPortFree(t *testing.T, port string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !devPortInUse(port) {
			time.Sleep(500 * time.Millisecond)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("dev port %s still in use after cleanup", port)
}

func devPortInUse(port string) bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "netstat -ano -p tcp").CombinedOutput()
		if err != nil {
			return true
		}
		needle := ":" + port
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(strings.ToUpper(line), "LISTENING") && strings.Contains(line, needle) {
				return true
			}
		}
		return false
	}
	out, err := exec.Command("lsof", "-ti", "tcp:"+port).CombinedOutput()
	if err != nil {
		return len(strings.TrimSpace(string(out))) > 0
	}
	return len(strings.Fields(string(out))) > 0
}

func waitForNuxtDevBoot(port, logPath string, timeout time.Duration) (ready bool, fatalReason string) {
	deadline := time.Now().Add(timeout)
	probeURL := "http://127.0.0.1:" + port + "/"
	for time.Now().Before(deadline) {
		if logPath != "" {
			data, err := os.ReadFile(logPath)
			if err == nil {
				state := evaluateNuxtDevBootLog(string(data), port, devPortInUse)
				if state.fatal {
					return false, state.reason
				}
			}
		}
		resp, err := http.Get(probeURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusInternalServerError {
				return true, ""
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false, ""
}

func assertNuxtDevStarted(t *testing.T, port string, logPath string) {
	t.Helper()
	ready, fatalReason := waitForNuxtDevBoot(port, logPath, 120*time.Second)
	if fatalReason != "" {
		dumpDevLogTail(t, logPath)
		t.Fatalf("web dev on port %s failed; %s (log=%s)", port, fatalReason, logPath)
	}
	if ready {
		return
	}
	dumpDevLogTail(t, logPath)
	t.Fatalf("web dev on port %s did not finish booting within timeout (log=%s)", port, logPath)
}

func dumpDevLogTail(t *testing.T, logPath string) {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Logf("dev log %s unavailable: %v", logPath, err)
		return
	}
	const maxTail = 4000
	text := string(data)
	if len(text) > maxTail {
		text = text[len(text)-maxTail:]
	}
	t.Logf("dev log tail (%s):\n%s", logPath, text)
}
