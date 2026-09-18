//go:build !windows

package studio

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readProcCmdline(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return ""
	}
	return readProcCmdlineFromBytes(data)
}

func readProcExe(pid int) string {
	if pid <= 0 {
		return ""
	}
	target, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return ""
	}
	norm, ok := normalizePathForComparison(target)
	if !ok {
		return target
	}
	return norm
}

func readProcParentPID(pid int) int {
	if pid <= 0 {
		return 0
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 4 {
		return 0
	}
	ppid, err := strconv.Atoi(fields[3])
	if err != nil {
		return 0
	}
	return ppid
}
