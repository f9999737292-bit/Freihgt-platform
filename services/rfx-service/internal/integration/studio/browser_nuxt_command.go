package studio

import (
	"context"
	"fmt"
	"os/exec"
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
