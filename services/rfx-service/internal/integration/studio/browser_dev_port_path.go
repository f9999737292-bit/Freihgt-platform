package studio

import (
	"path/filepath"
	"runtime"
	"strings"
)

func normalizePathForComparison(path string) (string, bool) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", false
	}
	cleaned := filepath.Clean(trimmed)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned, true
	}
	evaluated, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, true
	}
	return filepath.Clean(evaluated), true
}

func pathWithinRoot(path, root string) bool {
	normPath, ok := normalizePathForComparison(path)
	if !ok {
		return false
	}
	normRoot, ok := normalizePathForComparison(root)
	if !ok {
		return false
	}
	if runtime.GOOS == "windows" {
		normPath = strings.ToLower(normPath)
		normRoot = strings.ToLower(normRoot)
	}
	if normPath == normRoot {
		return true
	}
	sep := string(filepath.Separator)
	prefix := normRoot + sep
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(normPath, prefix)
	}
	return strings.HasPrefix(normPath, prefix)
}

func expectedNuxtAppDirs(worktreeRoot string) (adminAppDir, procAppDir string, ok bool) {
	root, okRoot := normalizePathForComparison(worktreeRoot)
	if !okRoot {
		return "", "", false
	}
	return filepath.Join(root, "apps", "web-admin"),
		filepath.Join(root, "apps", "web-procurement"),
		true
}

func nuxtPathWithinExpectedApps(nuxtPath, worktreeRoot string) bool {
	adminAppDir, procAppDir, ok := expectedNuxtAppDirs(worktreeRoot)
	if !ok {
		return false
	}
	return pathWithinRoot(nuxtPath, adminAppDir) || pathWithinRoot(nuxtPath, procAppDir)
}

func worktreePathInCommandLine(commandLine, worktreeRoot string) bool {
	normRoot, ok := normalizePathForComparison(worktreeRoot)
	if !ok {
		return false
	}
	candidates := []string{normRoot, filepath.ToSlash(normRoot)}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, strings.ToLower(normRoot))
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.Contains(commandLine, candidate) {
			return true
		}
	}
	return false
}
