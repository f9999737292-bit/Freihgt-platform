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

// linkNuxtBuildDirAt exposes isolated Nuxt buildDir at appDir/.nuxt so Vite can
// resolve tsconfig extends on clean CI checkouts.
func linkNuxtBuildDirAt(appDir, buildDir string) error {
	linkPath := filepath.Join(appDir, ".nuxt")
	if fi, err := os.Lstat(linkPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(linkPath)
			if readErr == nil && filepath.Clean(target) == filepath.Clean(buildDir) {
				return nil
			}
		}
		if err := os.RemoveAll(linkPath); err != nil {
			return fmt.Errorf("remove existing nuxt link path %s: %w", linkPath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(buildDir, linkPath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", linkPath, buildDir, err)
	}
	return nil
}

func unlinkNuxtBuildDirAt(appDir string) error {
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
	return os.Remove(linkPath)
}

func linkIsolatedNuxtBuildDir(appLabel, buildDir string) error {
	appDir, err := nuxtAppDir(appLabel)
	if err != nil {
		return err
	}
	return linkNuxtBuildDirAt(appDir, buildDir)
}

func unlinkIsolatedNuxtBuildDir(appLabel string) error {
	appDir, err := nuxtAppDir(appLabel)
	if err != nil {
		return err
	}
	return unlinkNuxtBuildDirAt(appDir)
}
