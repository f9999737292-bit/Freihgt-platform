//go:build integration

package studio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

type nuxtDevLaunch struct {
	appLabel string
	buildDir string
	linked   bool
	released bool
}

func (l *nuxtDevLaunch) release() error {
	if !isTaskOwnedNuxtBuildDirPath(l.buildDir) {
		return fmt.Errorf("refusing to release non task-owned build dir: %s", l.buildDir)
	}
	if l.linked {
		appDir, err := nuxtAppDir(l.appLabel)
		if err != nil {
			return err
		}
		if err := unlinkNuxtBuildDirAt(appDir, l.buildDir); err != nil {
			return err
		}
	}
	return removeGeneratedPathBounded(l.buildDir)
}

func releaseNuxtDevLaunchBounded(launch *nuxtDevLaunch) lockReleaseState {
	return boundedLockRelease(func() error {
		return launch.release()
	}, nuxtLockRetryDeadline, nuxtLockRetryInterval)
}

func (l *nuxtDevLaunch) releaseImmediate(t *testing.T) {
	t.Helper()
	if l == nil || l.released || l.buildDir == "" {
		return
	}
	state := releaseNuxtDevLaunchBounded(l)
	if !state.released && state.lastErr != nil {
		t.Logf("immediate nuxt launch cleanup %s: %v (attempts=%d deadline=%v)", l.buildDir, state.lastErr, state.attempts, state.deadlineHit)
	} else {
		l.released = true
	}
}

func registerNuxtDevLaunchCleanup(t *testing.T, launch *nuxtDevLaunch) {
	t.Helper()
	t.Cleanup(func() {
		if launch.released {
			return
		}
		state := releaseNuxtDevLaunchBounded(launch)
		if !state.released && state.lastErr != nil {
			t.Logf("cleanup isolated nuxt build dir %s: %v (attempts=%d deadline=%v)", launch.buildDir, state.lastErr, state.attempts, state.deadlineHit)
		} else {
			launch.released = true
		}
	})
}

func prepareNuxtDevLaunch(t *testing.T, appLabel, port string) *nuxtDevLaunch {
	t.Helper()
	buildDir, err := isolatedNuxtBuildDir(appLabel, port)
	if err != nil {
		t.Fatalf("isolated nuxt build dir for %s:%s: %v", appLabel, port, err)
	}
	if !nuxtBuildDirWithinAllowedRoot(buildDir) {
		t.Fatalf("isolated nuxt build dir outside allowed temp root: %s", buildDir)
	}
	appDir, err := nuxtAppDir(appLabel)
	if err != nil {
		t.Fatalf("nuxt app dir for %s:%s: %v", appLabel, port, err)
	}
	launch := &nuxtDevLaunch{appLabel: appLabel, buildDir: buildDir}
	if hasRealNuxtDirectory(appDir) {
		t.Logf("real .nuxt directory at %s; using NUXT_E2E_BUILD_DIR=%s without symlink", appDir, buildDir)
	} else if err := linkIsolatedNuxtBuildDir(appLabel, buildDir); err != nil {
		t.Fatalf("link isolated nuxt build dir for %s:%s: %v", appLabel, port, err)
	} else {
		launch.linked = true
	}
	registerNuxtDevLaunchCleanup(t, launch)
	return launch
}

func (l *nuxtDevLaunch) env() []string {
	return []string{"NUXT_E2E_BUILD_DIR=" + l.buildDir}
}

func appendNuxtDevEnv(base []string, launch *nuxtDevLaunch) []string {
	return append(base, launch.env()...)
}

func bootNuxtDevApp(t *testing.T, appLabel, port string, baseEnv []string) (*exec.Cmd, context.CancelFunc, *os.File, *nuxtDevLaunch) {
	t.Helper()
	const maxAttempts = 2
	var (
		lastLogPath  string
		firstFailure string
		lastLaunch   *nuxtDevLaunch
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			if lastLaunch != nil {
				lastLaunch.releaseImmediate(t)
				lastLaunch = nil
			}
			stopNuxtDevProcess(t, nil, port)
			ensureDevPortFree(t, port)
			if firstFailure != "" {
				t.Logf("retry nuxt dev boot app=%s port=%s attempt=%d after first failure: %s", appLabel, port, attempt, firstFailure)
			} else {
				t.Logf("retry nuxt dev boot app=%s port=%s attempt=%d", appLabel, port, attempt)
			}
		}
		launch := prepareNuxtDevLaunch(t, appLabel, port)
		lastLaunch = launch
		ctx, cancel := context.WithCancel(context.Background())
		env := appendNuxtDevEnv(baseEnv, launch)
		cmd, logFile := startNuxtDevCommand(t, ctx, appLabel, port, env)
		lastLogPath = logFile.Name()
		ready, fatalReason := waitForNuxtDevBoot(port, lastLogPath, 120*time.Second)
		if fatalReason != "" {
			stopNuxtDevProcess(t, cmd, port)
			cancel()
			_ = logFile.Close()
			dumpDevLogTail(t, lastLogPath)
			t.Fatalf("web dev on port %s failed; %s (log=%s)", port, fatalReason, lastLogPath)
		}
		if ready {
			return cmd, cancel, logFile, launch
		}
		if firstFailure == "" {
			firstFailure = fmt.Sprintf("boot probe timed out (log=%s)", lastLogPath)
		}
		stopNuxtDevProcess(t, cmd, port)
		cancel()
		_ = logFile.Close()
	}
	if firstFailure != "" {
		t.Logf("first nuxt dev boot failure preserved: %s", firstFailure)
	}
	dumpDevLogTail(t, lastLogPath)
	t.Fatalf("web dev on port %s did not finish booting after %d attempts (log=%s)", port, maxAttempts, lastLogPath)
	return nil, nil, nil, nil
}
