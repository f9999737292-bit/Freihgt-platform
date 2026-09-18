package studio

import (
	"path/filepath"
	"testing"
)

func TestResolveCommandNuxtPath_absoluteWithinApp(t *testing.T) {
	adminApp := filepath.Join(testWorktree, "apps", "web-admin")
	nuxtPath := filepath.Join(adminApp, "node_modules", "nuxt", "bin", "nuxt.mjs")
	resolved, ok := resolveCommandNuxtPath(`node "`+nuxtPath+`" dev --port 3022`, "")
	if !ok {
		t.Fatal("expected absolute nuxt path to resolve")
	}
	if !nuxtPathWithinExpectedApps(resolved, testWorktree) {
		t.Fatalf("resolved path %q should be within expected apps", resolved)
	}
}

func TestResolveCommandNuxtPath_relativeRequiresCwd(t *testing.T) {
	if _, ok := resolveCommandNuxtPath(`node ./node_modules/nuxt/bin/nuxt.mjs dev --port 3022`, ""); ok {
		t.Fatal("relative nuxt path without cwd must not resolve")
	}
}
