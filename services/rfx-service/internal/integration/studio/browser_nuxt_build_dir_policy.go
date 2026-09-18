package studio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	nuxtLockRetryInterval = 200 * time.Millisecond
	nuxtLockRetryDeadline = 5 * time.Second
)

func isolatedNuxtBuildDir(appLabel, port string) (string, error) {
	safeApp := strings.NewReplacer(" ", "-", "..", "-").Replace(appLabel)
	safePort := strings.NewReplacer(" ", "-", "..", "-").Replace(port)
	prefix := fmt.Sprintf("rfx-nuxt-%s-%s-", safeApp, safePort)
	return os.MkdirTemp("", prefix)
}

func nuxtBuildDirWithinAllowedRoot(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	tempRoot := os.TempDir()
	absTemp, err := filepath.Abs(tempRoot)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absTemp, absDir)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

func removeGeneratedPathBounded(path string) error {
	if !nuxtBuildDirWithinAllowedRoot(path) {
		return fmt.Errorf("refusing to remove path outside temp root: %s", path)
	}
	return os.RemoveAll(path)
}

type lockReleaseState struct {
	attempts    int
	released    bool
	lastErr     error
	deadlineHit bool
}

func boundedLockRelease(release func() error, deadline time.Duration, interval time.Duration) lockReleaseState {
	state := lockReleaseState{}
	deadlineAt := time.Now().Add(deadline)
	for time.Now().Before(deadlineAt) {
		state.attempts++
		err := release()
		if err == nil {
			state.released = true
			return state
		}
		state.lastErr = err
		if !isTransientPathBusy(err) {
			return state
		}
		time.Sleep(interval)
	}
	state.deadlineHit = true
	return state
}

func isTransientPathBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "ebusy") ||
		strings.Contains(msg, "resource busy") ||
		strings.Contains(msg, "being used by another process")
}

func nuxtAppDir(appLabel string) (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	switch appLabel {
	case "web-admin":
		return filepath.Join(root, "apps", "web-admin"), nil
	case "web-procurement":
		return filepath.Join(root, "apps", "web-procurement"), nil
	default:
		return "", fmt.Errorf("unsupported nuxt app label %q", appLabel)
	}
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "infrastructure", "migrations")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found from %s", wd)
		}
		dir = parent
	}
}

func resolveSymlinkTarget(linkPath string) (string, error) {
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return filepath.Clean(target), nil
	}
	return filepath.Clean(abs), nil
}

func sameBuildDirTarget(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return absA == absB
}

func createNuxtSymlink(linkPath, buildDir string) error {
	if err := os.Symlink(buildDir, linkPath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", linkPath, buildDir, err)
	}
	return nil
}

// linkNuxtBuildDirAt exposes isolated Nuxt buildDir at appDir/.nuxt so Vite can
// resolve tsconfig extends on clean CI checkouts.
func linkNuxtBuildDirAt(appDir, buildDir string) error {
	if !nuxtBuildDirWithinAllowedRoot(buildDir) {
		return fmt.Errorf("refusing to link non task-owned build dir: %s", buildDir)
	}
	linkPath := filepath.Join(appDir, ".nuxt")
	fi, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return createNuxtSymlink(linkPath, buildDir)
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, readErr := resolveSymlinkTarget(linkPath)
		if readErr != nil {
			return fmt.Errorf("existing .nuxt symlink unreadable at %s: %w", linkPath, readErr)
		}
		if sameBuildDirTarget(target, buildDir) {
			return nil
		}
		if nuxtBuildDirWithinAllowedRoot(target) {
			if err := os.Remove(linkPath); err != nil {
				return fmt.Errorf("remove stale task-owned nuxt link %s: %w", linkPath, err)
			}
			return createNuxtSymlink(linkPath, buildDir)
		}
		return fmt.Errorf("existing .nuxt symlink at %s targets %s (refusing replace with %s)", linkPath, target, buildDir)
	}
	return fmt.Errorf("existing .nuxt path at %s is not a task-owned symlink (refusing modify)", linkPath)
}

func hasRealNuxtDirectory(appDir string) bool {
	linkPath := filepath.Join(appDir, ".nuxt")
	fi, err := os.Lstat(linkPath)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink == 0
}

func unlinkNuxtBuildDirAt(appDir, expectedBuildDir string) error {
	linkPath := filepath.Join(appDir, ".nuxt")
	fi, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	target, readErr := resolveSymlinkTarget(linkPath)
	if readErr != nil {
		return fmt.Errorf("refusing to remove .nuxt symlink at %s: %w", linkPath, readErr)
	}
	if !sameBuildDirTarget(target, expectedBuildDir) {
		return nil
	}
	if !nuxtBuildDirWithinAllowedRoot(expectedBuildDir) {
		return fmt.Errorf("refusing to unlink .nuxt with target outside temp root: %s", target)
	}
	return os.Remove(linkPath)
}

func linkIsolatedNuxtBuildDir(appLabel, buildDir string) error {
	appDir, err := nuxtAppDir(appLabel)
	if err != nil {
		return err
	}
	return linkNuxtBuildDirAt(appDir, buildDir)
}

func unlinkIsolatedNuxtBuildDir(appLabel, expectedBuildDir string) error {
	appDir, err := nuxtAppDir(appLabel)
	if err != nil {
		return err
	}
	return unlinkNuxtBuildDirAt(appDir, expectedBuildDir)
}

const taskOwnedNuxtBuildDirPrefix = "rfx-nuxt-"

func isTaskOwnedNuxtBuildDirName(name string) bool {
	return strings.HasPrefix(name, taskOwnedNuxtBuildDirPrefix)
}

func isTaskOwnedNuxtBuildDirPath(path string) bool {
	if !nuxtBuildDirWithinAllowedRoot(path) {
		return false
	}
	return isTaskOwnedNuxtBuildDirName(filepath.Base(path))
}

func releaseNuxtDevLaunchAt(appDir, buildDir string) error {
	if !isTaskOwnedNuxtBuildDirPath(buildDir) {
		return fmt.Errorf("refusing to release non task-owned build dir: %s", buildDir)
	}
	if err := unlinkNuxtBuildDirAt(appDir, buildDir); err != nil {
		return err
	}
	return removeGeneratedPathBounded(buildDir)
}

// releaseNuxtDevLaunch unlinks the app .nuxt symlink before removing the temp build dir.
func releaseNuxtDevLaunch(appLabel, buildDir string) error {
	appDir, err := nuxtAppDir(appLabel)
	if err != nil {
		return err
	}
	return releaseNuxtDevLaunchAt(appDir, buildDir)
}

func listTaskOwnedTempBuildDirs() ([]string, error) {
	tempRoot := os.TempDir()
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() || !isTaskOwnedNuxtBuildDirName(entry.Name()) {
			continue
		}
		path := filepath.Join(tempRoot, entry.Name())
		if isTaskOwnedNuxtBuildDirPath(path) {
			dirs = append(dirs, path)
		}
	}
	return dirs, nil
}
