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
