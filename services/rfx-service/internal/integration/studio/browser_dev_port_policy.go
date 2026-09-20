package studio

import (
	"fmt"
	"path/filepath"
	"strings"
)

var scopedDevPorts = map[string]struct{}{
	"3020": {},
	"3022": {},
	"3023": {},
	"3031": {},
}

type portProcessInfo struct {
	PID         int
	ProcessName string
	CommandLine string
	Cwd         string
	ParentPID   int
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

func isNodeProcessName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "node.exe", "node":
		return true
	default:
		return false
	}
}

func extractNuxtEntryPath(commandLine string) (path string, isRelative bool, found bool) {
	replaced := strings.NewReplacer("\"", " ", "'", " ").Replace(commandLine)
	for _, token := range strings.Fields(replaced) {
		if !strings.Contains(strings.ToLower(token), "nuxt.mjs") {
			continue
		}
		cleaned := strings.Trim(token, `"'`)
		return cleaned, !filepath.IsAbs(cleaned), true
	}
	return "", false, false
}

func pnpmFilterPackage(commandLine string) (string, bool) {
	replaced := strings.NewReplacer("\"", " ", "'", " ").Replace(commandLine)
	fields := strings.Fields(replaced)
	for i, field := range fields {
		if strings.EqualFold(field, "--filter") || strings.EqualFold(field, "-F") {
			if i+1 >= len(fields) {
				return "", false
			}
			return strings.Trim(fields[i+1], `"'`), true
		}
	}
	return "", false
}

func isPnpmNuxtDevLauncher(commandLine string) bool {
	cmd := strings.ToLower(commandLine)
	return strings.Contains(cmd, "pnpm") &&
		strings.Contains(cmd, "exec") &&
		strings.Contains(cmd, "nuxt") &&
		strings.Contains(cmd, "dev")
}

func expectedPackageForAppDir(appDir string) (string, bool) {
	switch filepath.Base(appDir) {
	case "web-admin":
		return "@freight-platform/web-admin", true
	case "web-procurement":
		return "@freight-platform/web-procurement", true
	default:
		return "", false
	}
}

func expectedAppDirForPort(port, worktreeRoot string) (string, bool) {
	root, ok := normalizePathForComparison(worktreeRoot)
	if !ok {
		return "", false
	}
	switch port {
	case "3020", "3022":
		return filepath.Join(root, "apps", "web-admin"), true
	case "3023", "3031":
		return filepath.Join(root, "apps", "web-procurement"), true
	default:
		return "", false
	}
}

func resolveOwnedNuxtPath(port string, proc portProcessInfo, worktreeRoot string) (path string, ok bool, reason string) {
	_, isRelative, found := extractNuxtEntryPath(proc.CommandLine)
	if !found {
		return "", false, "nuxt entry not found in argv"
	}
	expectedApp, hasApp := expectedAppDirForPort(port, worktreeRoot)
	if !hasApp {
		return "", false, "unknown port app mapping"
	}
	normExpectedApp, appOK := normalizePathForComparison(expectedApp)
	if !appOK {
		return "", false, "expected app dir invalid"
	}
	if !isRelative {
		resolved, resolvedOK := resolveCommandNuxtPath(proc.CommandLine, "")
		if !resolvedOK {
			return "", false, "unable to resolve absolute nuxt entry path"
		}
		if pathWithinRoot(resolved, normExpectedApp) {
			return resolved, true, "absolute nuxt path within expected app"
		}
		return "", false, "absolute nuxt path outside expected app for port"
	}
	cwd, cwdOK := verifiedProcessCwd(proc)
	if !cwdOK {
		return "", false, "relative nuxt argv without verified cwd"
	}
	normWorktree, worktreeOK := normalizePathForComparison(worktreeRoot)
	if !worktreeOK || !pathWithinRoot(cwd, normWorktree) {
		return "", false, "cwd outside worktree"
	}
	if pathWithinRoot(cwd, normExpectedApp) {
		if resolved, resolvedOK := resolveCommandNuxtPath(proc.CommandLine, cwd); resolvedOK {
			if pathWithinRoot(resolved, normExpectedApp) {
				return resolved, true, "resolved from verified cwd within expected app"
			}
		}
	}
	if cwd == normWorktree {
		if resolved, resolvedOK := resolveCommandNuxtPath(proc.CommandLine, normExpectedApp); resolvedOK {
			if pathWithinRoot(resolved, normExpectedApp) {
				return resolved, true, "resolved from expected app dir for port"
			}
		}
	}
	return "", false, "unable to resolve relative nuxt path within expected app"
}

func resolveCommandNuxtPath(commandLine, cwd string) (string, bool) {
	entryPath, isRelative, found := extractNuxtEntryPath(commandLine)
	if !found {
		return "", false
	}
	if filepath.IsAbs(entryPath) {
		norm, ok := normalizePathForComparison(entryPath)
		return norm, ok
	}
	if !isRelative || strings.TrimSpace(cwd) == "" {
		return "", false
	}
	joined := filepath.Join(cwd, entryPath)
	norm, ok := normalizePathForComparison(joined)
	return norm, ok
}

func verifiedProcessCwd(proc portProcessInfo) (string, bool) {
	if strings.TrimSpace(proc.Cwd) != "" {
		norm, ok := normalizePathForComparison(proc.Cwd)
		return norm, ok
	}
	cwd := lookupProcessCwd(proc.PID)
	if strings.TrimSpace(cwd) == "" {
		return "", false
	}
	norm, ok := normalizePathForComparison(cwd)
	return norm, ok
}

func isTaskOwnedPnpmNuxtLauncher(port string, proc portProcessInfo, worktreeRoot string) (bool, string) {
	if !isPnpmNuxtDevLauncher(proc.CommandLine) || !commandLineMatchesNuxtDevPort(proc.CommandLine, port) {
		return false, "not a pnpm nuxt dev launcher"
	}
	filter, ok := pnpmFilterPackage(proc.CommandLine)
	if !ok {
		return false, "pnpm filter missing"
	}
	adminAppDir, procAppDir, ok := expectedNuxtAppDirs(worktreeRoot)
	if !ok {
		return false, "worktree root invalid"
	}
	expectedAdmin, _ := expectedPackageForAppDir(adminAppDir)
	expectedProc, _ := expectedPackageForAppDir(procAppDir)
	if filter != expectedAdmin && filter != expectedProc {
		return false, "pnpm filter does not match expected app"
	}
	// Corepack/pnpm launchers often run with a global cwd; package filter + port bind task ownership.
	return true, "pnpm filter matches expected task app"
}

func isTaskOwnedNuxtCliDev(port string, proc portProcessInfo, worktreeRoot string) (bool, string) {
	cmd := strings.ToLower(proc.CommandLine)
	if (!strings.Contains(cmd, "@nuxt/cli") && !strings.Contains(cmd, "@nuxt+cli")) ||
		!strings.Contains(cmd, "dev") {
		return false, "not an @nuxt/cli dev process"
	}
	if !commandLineMatchesNuxtDevPort(proc.CommandLine, port) {
		return false, "command does not match nuxt dev port"
	}
	if worktreePathInCommandLine(proc.CommandLine, worktreeRoot) {
		return true, "@nuxt/cli dev references worktree path"
	}
	cwd, cwdOK := verifiedProcessCwd(proc)
	if cwdOK && pathWithinRoot(cwd, worktreeRoot) {
		return true, "@nuxt/cli dev cwd within worktree"
	}
	return false, "@nuxt/cli dev outside worktree"
}

func isTaskOwnedDirectNuxtDev(port string, proc portProcessInfo, worktreeRoot string) (bool, string) {
	if !commandLineMatchesNuxtDevPort(proc.CommandLine, port) {
		return false, "command does not match nuxt dev port"
	}
	nuxtPath, ok, reason := resolveOwnedNuxtPath(port, proc, worktreeRoot)
	if !ok {
		return false, reason
	}
	expectedApp, hasApp := expectedAppDirForPort(port, worktreeRoot)
	if !hasApp {
		return false, "unknown port app mapping"
	}
	if !pathWithinRoot(nuxtPath, expectedApp) {
		return false, "resolved nuxt path outside expected app for port"
	}
	return true, reason
}

func isTaskOwnedStaleNuxtDevListener(port, processName, commandLine, worktreeRoot string) bool {
	proc := portProcessInfo{
		ProcessName: processName,
		CommandLine: commandLine,
	}
	owned, _ := classifyTaskOwnedNuxtDev(port, proc, worktreeRoot)
	return owned
}

func isTaskOwnedStalePnpmNuxtLauncher(port, processName, commandLine, worktreeRoot string) bool {
	proc := portProcessInfo{
		ProcessName: processName,
		CommandLine: commandLine,
	}
	owned, _ := isTaskOwnedPnpmNuxtLauncher(port, proc, worktreeRoot)
	return owned
}

func classifyTaskOwnedNuxtDev(port string, proc portProcessInfo, worktreeRoot string) (owned bool, reason string) {
	if !isScopedDevPort(port) {
		return false, "port out of scope"
	}
	if isProtectedPortProcessName(proc.ProcessName) {
		return false, "protected process"
	}
	if !isNodeProcessName(proc.ProcessName) {
		return false, "not a node process"
	}
	if !commandLineMatchesNuxtDevPort(proc.CommandLine, port) {
		return false, "command does not match nuxt dev port"
	}
	if owned, reason := isTaskOwnedPnpmNuxtLauncher(port, proc, worktreeRoot); owned {
		return true, reason
	}
	if owned, reason := isTaskOwnedNuxtCliDev(port, proc, worktreeRoot); owned {
		return true, reason
	}
	return isTaskOwnedDirectNuxtDev(port, proc, worktreeRoot)
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
	if owned, ownedReason := classifyTaskOwnedNuxtDev(port, proc, worktreeRoot); owned {
		return true, false, ownedReason
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
