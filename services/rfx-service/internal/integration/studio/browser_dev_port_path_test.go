package studio

import (
	"path/filepath"
	"testing"
)

func TestResolveCommandNuxtPath_absoluteWithinApp(t *testing.T) {
	adminApp := testAdminAppDir(t)
	nuxtPath := filepath.Join(adminApp, "node_modules", "nuxt", "bin", "nuxt.mjs")
	resolved, ok := resolveCommandNuxtPath(`node "`+nuxtPath+`" dev --port 3022`, "")
	if !ok {
		t.Fatal("expected absolute nuxt path to resolve")
	}
	if !pathWithinRoot(resolved, adminApp) {
		t.Fatalf("resolved path %q should be within expected app %q", resolved, adminApp)
	}
}

func TestResolveCommandNuxtPath_relativeRequiresCwd(t *testing.T) {
	if _, ok := resolveCommandNuxtPath(`node ./node_modules/nuxt/bin/nuxt.mjs dev --port 3022`, ""); ok {
		t.Fatal("relative nuxt path without cwd must not resolve")
	}
}

func TestResolveOwnedNuxtPath_relativeFromWorktreeRootForPort3023(t *testing.T) {
	worktree := testWorktreeRoot(t)
	proc := portProcessInfo{
		PID:         6295,
		ProcessName: "node",
		CommandLine: `node ./node_modules/.bin/../nuxt/bin/nuxt.mjs dev --port 3023 --host 127.0.0.1`,
		Cwd:         worktree,
	}
	path, ok, reason := resolveOwnedNuxtPath("3023", proc, worktree)
	if !ok || reason == "" {
		t.Fatalf("expected owned path resolution, got ok=%v reason=%q", ok, reason)
	}
	expectedApp := testProcurementAppDir(t)
	if !pathWithinRoot(path, expectedApp) {
		t.Fatalf("resolved path %q should be within %q", path, expectedApp)
	}
}
