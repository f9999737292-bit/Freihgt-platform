//go:build integration

package studio

import (
	"testing"
)

type nuxtDevLaunch struct {
	buildDir string
}

func prepareNuxtDevLaunch(t *testing.T, appLabel, port string) nuxtDevLaunch {
	t.Helper()
	buildDir, err := isolatedNuxtBuildDir(appLabel, port)
	if err != nil {
		t.Fatalf("isolated nuxt build dir for %s:%s: %v", appLabel, port, err)
	}
	if !nuxtBuildDirWithinAllowedRoot(buildDir) {
		t.Fatalf("isolated nuxt build dir outside allowed temp root: %s", buildDir)
	}
	if err := linkIsolatedNuxtBuildDir(appLabel, buildDir); err != nil {
		t.Fatalf("link isolated nuxt build dir for %s:%s: %v", appLabel, port, err)
	}
	t.Cleanup(func() {
		if err := unlinkIsolatedNuxtBuildDir(appLabel); err != nil {
			t.Logf("unlink isolated nuxt build dir for %s: %v", appLabel, err)
		}
	})
	t.Cleanup(func() {
		state := boundedLockRelease(func() error {
			return removeGeneratedPathBounded(buildDir)
		}, nuxtLockRetryDeadline, nuxtLockRetryInterval)
		if !state.released && state.lastErr != nil {
			t.Logf("cleanup isolated nuxt build dir %s: %v (attempts=%d deadline=%v)", buildDir, state.lastErr, state.attempts, state.deadlineHit)
		}
	})
	return nuxtDevLaunch{buildDir: buildDir}
}

func (l nuxtDevLaunch) env() []string {
	return []string{"NUXT_E2E_BUILD_DIR=" + l.buildDir}
}

func appendNuxtDevEnv(base []string, launch nuxtDevLaunch) []string {
	return append(base, launch.env()...)
}
