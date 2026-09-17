package studio

import (
	"fmt"
	"strings"
)

var scopedDevPorts = map[string]struct{}{
	"3022": {},
	"3023": {},
}

type portProcessInfo struct {
	PID         int
	ProcessName string
	CommandLine string
}

func isScopedDevPort(port string) bool {
	_, ok := scopedDevPorts[port]
	return ok
}

func isProtectedPortProcessName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "powershell.exe", "pwsh.exe", "cmd.exe", "cursor.exe", "code.exe",
		"docker.exe", "com.docker.backend.exe", "wsl.exe", "wslhost.exe",
		"svchost.exe", "system", "registry":
		return true
	default:
		return false
	}
}

func commandLineMatchesNuxtDevPort(commandLine, port string) bool {
	cmd := strings.ToLower(commandLine)
	hasNuxtDev := strings.Contains(cmd, "nuxt.mjs") ||
		(strings.Contains(cmd, "nuxt") && strings.Contains(cmd, "dev"))
	if !hasNuxtDev {
		return false
	}
	normalized := strings.NewReplacer("\"", " ", "'", " ").Replace(cmd)
	normalized = strings.Join(strings.Fields(normalized), " ")
	return strings.Contains(normalized, "--port "+port) ||
		strings.Contains(cmd, "--port="+port)
}

func isTaskOwnedWorktree(commandLine, worktreeRoot string) bool {
	root := strings.TrimSpace(worktreeRoot)
	if root != "" && strings.Contains(commandLine, root) {
		return true
	}
	cmd := strings.ToLower(commandLine)
	return strings.Contains(cmd, "web-admin") ||
		strings.Contains(cmd, "web-procurement") ||
		strings.Contains(cmd, "@freight-platform/web-admin") ||
		strings.Contains(cmd, "@freight-platform/web-procurement")
}

func isTaskOwnedStaleNuxtDevListener(port, processName, commandLine, worktreeRoot string) bool {
	if !isScopedDevPort(port) {
		return false
	}
	if isProtectedPortProcessName(processName) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(processName), "node.exe") {
		return false
	}
	if isTaskOwnedStalePnpmNuxtLauncher(port, processName, commandLine, worktreeRoot) {
		return true
	}
	if !commandLineMatchesNuxtDevPort(commandLine, port) {
		return false
	}
	return isTaskOwnedWorktree(commandLine, worktreeRoot)
}

func isTaskOwnedStalePnpmNuxtLauncher(port, processName, commandLine, worktreeRoot string) bool {
	if !isScopedDevPort(port) {
		return false
	}
	if isProtectedPortProcessName(processName) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(processName), "node.exe") {
		return false
	}
	cmd := strings.ToLower(commandLine)
	if !strings.Contains(cmd, "pnpm") || !strings.Contains(cmd, "exec") || !strings.Contains(cmd, "nuxt") {
		return false
	}
	if !commandLineMatchesNuxtDevPort(commandLine, port) {
		return false
	}
	return isTaskOwnedWorktree(commandLine, worktreeRoot)
}

func classifyDevPortListener(port string, proc portProcessInfo, worktreeRoot string) (kill bool, blocked bool, reason string) {
	if !isScopedDevPort(port) {
		return false, false, "port out of scope"
	}
	if proc.PID <= 0 {
		return false, true, "ambiguous pid"
	}
	if isProtectedPortProcessName(proc.ProcessName) {
		return false, true, fmt.Sprintf("protected process %q", proc.ProcessName)
	}
	if isTaskOwnedStaleNuxtDevListener(port, proc.ProcessName, proc.CommandLine, worktreeRoot) {
		return true, false, "task-owned stale nuxt dev"
	}
	return false, true, fmt.Sprintf("unknown listener owner process=%q cmd=%q", proc.ProcessName, sanitizeCommandLineForLog(proc.CommandLine, 160))
}

func sanitizeCommandLineForLog(commandLine string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 160
	}
	trimmed := strings.Join(strings.Fields(commandLine), " ")
	if len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen] + "..."
}
