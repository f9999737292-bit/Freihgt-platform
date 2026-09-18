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

func TestLinkNuxtBuildDirAt_exposesTsconfigForVite(t *testing.T) {
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-link-test-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	if err := linkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("link: %v", err)
	}
	t.Cleanup(func() { _ = unlinkNuxtBuildDirAt(appDir, buildDir) })
	tsconfig := []byte(`{"compilerOptions":{"strict":true}}`)
	if err := os.WriteFile(filepath.Join(buildDir, "tsconfig.json"), tsconfig, 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}
	linked, err := os.ReadFile(filepath.Join(appDir, ".nuxt", "tsconfig.json"))
	if err != nil {
		t.Fatalf("read linked tsconfig: %v", err)
	}
	if string(linked) != string(tsconfig) {
		t.Fatalf("expected linked tsconfig content, got %q", string(linked))
	}
}

func TestLinkNuxtBuildDirAt_idempotentForSameTarget(t *testing.T) {
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-link-idempotent")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	if err := linkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("first link: %v", err)
	}
	if err := linkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("second link should be idempotent: %v", err)
	}
	t.Cleanup(func() { _ = unlinkNuxtBuildDirAt(appDir, buildDir) })
}

func TestLinkNuxtBuildDirAt_blocksRealDirectory(t *testing.T) {
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-link-real-dir")
	if err := os.MkdirAll(filepath.Join(appDir, ".nuxt"), 0o755); err != nil {
		t.Fatalf("mkdir real .nuxt: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	marker := filepath.Join(appDir, ".nuxt", "preserve-me.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := linkNuxtBuildDirAt(appDir, buildDir); err == nil {
		t.Fatal("expected link against real directory to fail closed")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("real .nuxt contents must be preserved: %v", err)
	}
}

func TestLinkNuxtBuildDirAt_reclaimsStaleTaskOwnedSymlink(t *testing.T) {
	staleTarget, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("stale target: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(staleTarget) })
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3023")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-link-stale")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	linkPath := filepath.Join(appDir, ".nuxt")
	if err := os.Symlink(staleTarget, linkPath); err != nil {
		t.Fatalf("create stale link: %v", err)
	}
	if err := linkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("expected stale task-owned link to be reclaimed: %v", err)
	}
	t.Cleanup(func() { _ = unlinkNuxtBuildDirAt(appDir, buildDir) })
	target, err := resolveSymlinkTarget(linkPath)
	if err != nil {
		t.Fatalf("reclaimed link unreadable: %v", err)
	}
	if !sameBuildDirTarget(target, buildDir) {
		t.Fatalf("reclaimed link target=%q want=%q", target, buildDir)
	}
}

func TestLinkNuxtBuildDirAt_blocksForeignSymlinkOutsideTemp(t *testing.T) {
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3023")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-link-foreign-outside")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	root, rootErr := findRepoRoot()
	if rootErr != nil {
		t.Fatalf("repo root: %v", rootErr)
	}
	outside := filepath.Join(root, ".outside-rfx-nuxt-link-target")
	if nuxtBuildDirWithinAllowedRoot(outside) {
		t.Fatalf("test setup: outside target must not be under temp root, got %q", outside)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside target: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	linkPath := filepath.Join(appDir, ".nuxt")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("create foreign link: %v", err)
	}
	if err := linkNuxtBuildDirAt(appDir, buildDir); err == nil {
		t.Fatal("expected foreign symlink outside temp to be blocked")
	}
	target, err := resolveSymlinkTarget(linkPath)
	if err != nil {
		t.Fatalf("foreign link should remain readable: %v", err)
	}
	if !sameBuildDirTarget(target, outside) {
		t.Fatalf("foreign link target changed to %q", target)
	}
}

func TestUnlinkNuxtBuildDirAt_onlyOwnTaskOwnedLink(t *testing.T) {
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	foreignTarget, err := isolatedNuxtBuildDir("web-admin", "3023")
	if err != nil {
		t.Fatalf("foreign target: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(foreignTarget) })
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-unlink-own")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	if err := linkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := unlinkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("cleanup own link: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(appDir, ".nuxt")); !os.IsNotExist(err) {
		t.Fatalf("expected own link removed, got err=%v", err)
	}
	if err := os.Symlink(foreignTarget, filepath.Join(appDir, ".nuxt")); err != nil {
		t.Fatalf("create foreign link: %v", err)
	}
	if err := unlinkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("foreign link cleanup should noop without error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(appDir, ".nuxt")); err != nil {
		t.Fatalf("foreign link must be preserved: %v", err)
	}
}

func TestUnlinkNuxtBuildDirAt_preservesRealDirectory(t *testing.T) {
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-unlink-real")
	if err := os.MkdirAll(filepath.Join(appDir, ".nuxt"), 0o755); err != nil {
		t.Fatalf("mkdir real .nuxt: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	marker := filepath.Join(appDir, ".nuxt", "preserve-me.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := unlinkNuxtBuildDirAt(appDir, filepath.Join(os.TempDir(), "unused")); err != nil {
		t.Fatalf("cleanup real directory should noop: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("real .nuxt must remain: %v", err)
	}
}

func TestLinkNuxtBuildDirAt_rejectsOutsideTempBuildDir(t *testing.T) {
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-link-outside-temp")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	if err := linkNuxtBuildDirAt(appDir, `C:\outside\temp\builddir`); err == nil {
		t.Fatal("expected outside build dir to be rejected")
	}
}

func TestHasRealNuxtDirectory_detectsDirectoryNotSymlink(t *testing.T) {
	realAppDir := filepath.Join(os.TempDir(), "rfx-nuxt-real-dir-detect")
	if err := os.MkdirAll(filepath.Join(realAppDir, ".nuxt"), 0o755); err != nil {
		t.Fatalf("mkdir real .nuxt: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(realAppDir) })
	if !hasRealNuxtDirectory(realAppDir) {
		t.Fatal("expected real .nuxt directory to be detected")
	}
	symlinkAppDir := filepath.Join(os.TempDir(), "rfx-nuxt-symlink-dir-detect")
	if err := os.MkdirAll(symlinkAppDir, 0o755); err != nil {
		t.Fatalf("mkdir symlink app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(symlinkAppDir) })
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	if err := linkNuxtBuildDirAt(symlinkAppDir, buildDir); err != nil {
		t.Fatalf("link: %v", err)
	}
	t.Cleanup(func() { _ = unlinkNuxtBuildDirAt(symlinkAppDir, buildDir) })
	if hasRealNuxtDirectory(symlinkAppDir) {
		t.Fatal("expected symlink .nuxt not to count as real directory")
	}
}

func TestReleaseNuxtDevLaunch_unlinksBeforeRemovingTarget(t *testing.T) {
	buildDir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-release-order")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	linkPath := filepath.Join(appDir, ".nuxt")
	if err := linkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := releaseNuxtDevLaunchAt(appDir, buildDir); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		t.Fatalf("expected build dir removed, err=%v", err)
	}
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("expected .nuxt link removed before target deletion, err=%v", err)
	}
}

func TestReleaseNuxtDevLaunch_repeatedCleanupIsSafe(t *testing.T) {
	buildDir, err := isolatedNuxtBuildDir("web-procurement", "3023")
	if err != nil {
		t.Fatalf("build dir: %v", err)
	}
	appDir := filepath.Join(os.TempDir(), "rfx-nuxt-release-repeat")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(appDir) })
	if err := linkNuxtBuildDirAt(appDir, buildDir); err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := releaseNuxtDevLaunch("web-procurement", buildDir); err != nil {
		t.Fatalf("first release: %v", err)
	}
	if err := releaseNuxtDevLaunch("web-procurement", buildDir); err != nil {
		t.Fatalf("second release should be safe: %v", err)
	}
}

func TestReleaseNuxtDevLaunch_blocksForeignPath(t *testing.T) {
	root, rootErr := findRepoRoot()
	if rootErr != nil {
		t.Fatalf("repo root: %v", rootErr)
	}
	outside := filepath.Join(root, ".outside-rfx-nuxt-release")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	if err := releaseNuxtDevLaunch("web-admin", outside); err == nil {
		t.Fatal("expected foreign path release to be blocked")
	}
}

func TestReleaseNuxtDevLaunch_blocksNonTaskOwnedName(t *testing.T) {
	foreign := filepath.Join(os.TempDir(), "not-rfx-nuxt-dir")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatalf("mkdir foreign: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(foreign) })
	if err := releaseNuxtDevLaunch("web-admin", foreign); err == nil {
		t.Fatal("expected non task-owned temp name to be blocked")
	}
}

func TestListTaskOwnedTempBuildDirs_onlyRfxNuxtPrefix(t *testing.T) {
	dir, err := isolatedNuxtBuildDir("web-admin", "3022")
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	dirs, err := listTaskOwnedTempBuildDirs()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, path := range dirs {
		if path == dir {
			found = true
		}
		if !isTaskOwnedNuxtBuildDirPath(path) {
			t.Fatalf("listed non task-owned path %q", path)
		}
	}
	if !found {
		t.Fatalf("expected listed task-owned dir %q in %v", dir, dirs)
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
