package studio

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testWorktreeRoot(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return `D:\Projects\freight-platform-wt\rfx-scoring-browser-startup-race-v3.0d`
	}
	return "/home/runner/work/Freihgt-platform/Freihgt-platform"
}

func testAdminAppDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testWorktreeRoot(t), "apps", "web-admin")
}

func testProcurementAppDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testWorktreeRoot(t), "apps", "web-procurement")
}
