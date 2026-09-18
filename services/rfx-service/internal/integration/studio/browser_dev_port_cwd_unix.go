//go:build !windows

package studio

import (
	"fmt"
	"os"
	"path/filepath"
)

func lookupProcessCwd(pid int) string {
	if pid <= 0 {
		return ""
	}
	link := fmt.Sprintf("/proc/%d/cwd", pid)
	target, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		return ""
	}
	norm, ok := normalizePathForComparison(target)
	if !ok {
		return ""
	}
	return norm
}
