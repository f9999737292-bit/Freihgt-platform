package studio

import (
	"strings"
	"testing"
	"time"
)

func TestNuxtProcessLaunchRecord_completeAndValidate(t *testing.T) {
	record := &nuxtProcessLaunchRecord{
		LauncherPID:       100,
		PGID:              100,
		LauncherStarttime: 12345,
		AppLabel:          "web-admin",
		ExpectedPort:      "3022",
		WorktreeRoot:      testWorktreeRoot(t),
		AppRoot:           testAdminAppDir(t),
		BuildDir:          t.TempDir(),
		RunToken:          "token",
		CreatedAt:         time.Now().UTC(),
	}
	if !record.complete() {
		t.Fatal("expected complete launch record")
	}
	if err := record.validateForShutdown("3022"); err != nil {
		t.Fatalf("validateForShutdown: %v", err)
	}
}

func TestNuxtProcessLaunchRecord_missingRunTokenIncomplete(t *testing.T) {
	record := &nuxtProcessLaunchRecord{
		LauncherPID:       100,
		PGID:              100,
		LauncherStarttime: 12345,
		AppLabel:          "web-admin",
		ExpectedPort:      "3022",
		WorktreeRoot:      testWorktreeRoot(t),
		AppRoot:           testAdminAppDir(t),
		BuildDir:          t.TempDir(),
	}
	if record.complete() {
		t.Fatal("expected missing run token to make record incomplete")
	}
}

func TestNuxtProcessLaunchRecord_portMismatchRejected(t *testing.T) {
	record := validTestLaunchRecord(t, "3022")
	if err := record.validateForShutdown("3023"); err == nil || !strings.Contains(err.Error(), "port mismatch") {
		t.Fatalf("expected port mismatch error, got %v", err)
	}
}

func TestNuxtProcessLaunchRecord_invalidPGIDRejected(t *testing.T) {
	record := validTestLaunchRecord(t, "3022")
	record.PGID = 1
	if err := record.validateForShutdown("3022"); err == nil || !strings.Contains(err.Error(), "not task-owned") {
		t.Fatalf("expected invalid pgid error, got %v", err)
	}
}

func validTestLaunchRecord(t *testing.T, port string) *nuxtProcessLaunchRecord {
	t.Helper()
	return &nuxtProcessLaunchRecord{
		LauncherPID:       100,
		PGID:              100,
		LauncherStarttime: 12345,
		AppLabel:          "web-admin",
		ExpectedPort:      port,
		WorktreeRoot:      testWorktreeRoot(t),
		AppRoot:           testAdminAppDir(t),
		BuildDir:          t.TempDir(),
		RunToken:          "token",
		CreatedAt:         time.Now().UTC(),
	}
}
