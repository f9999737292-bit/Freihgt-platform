package studio

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func nuxtPackageFilter(appLabel string) (string, error) {
	switch appLabel {
	case "web-admin":
		return "@freight-platform/web-admin", nil
	case "web-procurement":
		return "@freight-platform/web-procurement", nil
	default:
		return "", fmt.Errorf("unsupported nuxt app label %q", appLabel)
	}
}

func newNuxtDevCommand(ctx context.Context, root string, filterPackage string, port string, env []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "pnpm",
		"--filter", filterPackage,
		"exec", "nuxt", "dev",
		"--port", port,
		"--host", "127.0.0.1",
	)
	cmd.Dir = root
	cmd.Env = env
	return cmd
}

func newNuxtPrepareCommand(ctx context.Context, root, filterPackage string, env []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "pnpm",
		"--filter", filterPackage,
		"exec", "nuxt", "prepare",
	)
	cmd.Dir = root
	cmd.Env = env
	return cmd
}

func envHasIsolatedNuxtBuildDir(env []string) bool {
	for _, entry := range env {
		if strings.HasPrefix(entry, "NUXT_E2E_BUILD_DIR=") {
			return strings.TrimPrefix(entry, "NUXT_E2E_BUILD_DIR=") != ""
		}
	}
	return false
}

func runNuxtPrepare(ctx context.Context, root, filterPackage string, env []string) error {
	if !envHasIsolatedNuxtBuildDir(env) {
		return nil
	}
	// Isolated temp build dirs: let `nuxt dev` prepare lazily. A separate prepare pass
	// races with dev startup on Windows and can EBUSY-lock the temp `dev` directory.
	_ = ctx
	_ = root
	_ = filterPackage
	return nil
}
