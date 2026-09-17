package studio

import (
	"context"
	"strings"
	"testing"
)

func TestNuxtPackageFilter_knownApps(t *testing.T) {
	admin, err := nuxtPackageFilter("web-admin")
	if err != nil || admin != "@freight-platform/web-admin" {
		t.Fatalf("web-admin filter: %q err=%v", admin, err)
	}
	proc, err := nuxtPackageFilter("web-procurement")
	if err != nil || proc != "@freight-platform/web-procurement" {
		t.Fatalf("web-procurement filter: %q err=%v", proc, err)
	}
}

func TestNuxtPackageFilter_unknownApp(t *testing.T) {
	if _, err := nuxtPackageFilter("unknown"); err == nil {
		t.Fatal("expected error for unknown app label")
	}
}

func TestNewNuxtDevCommand_usesPnpmFilterExec(t *testing.T) {
	cmd := newNuxtDevCommand(context.Background(), `D:\Projects\freight-platform-wt\rfx-scoring-browser-startup-race-v3.0d`, "@freight-platform/web-admin", "3022", nil)
	if cmd.Path == "" && len(cmd.Args) == 0 {
		t.Fatal("expected command args")
	}
	joined := strings.Join(cmd.Args, " ")
	for _, want := range []string{"pnpm", "--filter", "@freight-platform/web-admin", "exec", "nuxt", "dev", "--port", "3022", "--host", "127.0.0.1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command %q missing %q", joined, want)
		}
	}
	if cmd.Dir != `D:\Projects\freight-platform-wt\rfx-scoring-browser-startup-race-v3.0d` {
		t.Fatalf("unexpected cwd: %q", cmd.Dir)
	}
}
