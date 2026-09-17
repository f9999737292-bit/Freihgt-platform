package studio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsolatedNuxtBuildDir_uniqueSequentialPaths(t *testing.T) {
	first, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("first dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(first) })
	second, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("second dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(second) })
	if first == second {
		t.Fatalf("expected distinct build dirs, got %q", first)
	}
	if !nuxtBuildDirWithinAllowedRoot(first) || !nuxtBuildDirWithinAllowedRoot(second) {
		t.Fatalf("build dirs must stay under temp root")
	}
}

func TestNuxtBuildDirWithinAllowedRoot_rejectsOutsideTemp(t *testing.T) {
	outside := filepath.Join(string(filepath.Separator), "definitely-not-temp-rfx-nuxt")
	if nuxtBuildDirWithinAllowedRoot(outside) {
		t.Fatalf("expected outside path to be rejected")
	}
}

func TestRemoveGeneratedPathBounded_onlyTempRoot(t *testing.T) {
	dir, err := isolatedNuxtBuildDir("web-procurement", "3023")
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	if err := removeGeneratedPathBounded(dir); err != nil {
		t.Fatalf("remove allowed temp dir: %v", err)
	}
	if err := removeGeneratedPathBounded(`C:\outside\temp\dir`); err == nil {
		t.Fatalf("expected refusal for path outside temp root")
	}
}

func TestBoundedLockRelease_transientThenSuccess(t *testing.T) {
	attempts := 0
	state := boundedLockRelease(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("EBUSY: resource busy or locked")
		}
		return nil
	}, 2*time.Second, 50*time.Millisecond)
	if !state.released || state.attempts < 3 {
		t.Fatalf("expected transient recovery, got %+v attempts=%d", state, attempts)
	}
}

func TestBoundedLockRelease_persistentFailFast(t *testing.T) {
	state := boundedLockRelease(func() error {
		return errors.New("permission denied")
	}, time.Second, 50*time.Millisecond)
	if state.released || !strings.Contains(state.lastErr.Error(), "permission denied") {
		t.Fatalf("expected persistent failure, got %+v", state)
	}
}

func TestIsolatedNuxtBuildDir_handlesSpacesInWorktreePath(t *testing.T) {
	nested := filepath.Join(os.TempDir(), "rfx nuxt harness", "worktree")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(os.TempDir(), "rfx nuxt harness")) })
	dir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("dir with spaced temp parent context: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if !nuxtBuildDirWithinAllowedRoot(dir) {
		t.Fatalf("expected valid temp build dir, got %q", dir)
	}
}
