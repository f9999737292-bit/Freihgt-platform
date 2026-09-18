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
	fields, ok := parseProcStatFields(pid)
	if !ok || len(fields) < 4 {
		return 0
	}
	ppid, err := strconv.Atoi(fields[3])
	if err != nil {
		return 0
	}
	return ppid
}

func readProcPGID(pid int) (int, bool) {
	fields, ok := parseProcStatFields(pid)
	if !ok || len(fields) < 5 {
		return 0, false
	}
	pgid, err := strconv.Atoi(fields[4])
	if err != nil || pgid <= 0 {
		return 0, false
	}
	return pgid, true
}

func readProcStarttime(pid int) (uint64, bool) {
	fields, ok := parseProcStatFields(pid)
	if !ok || len(fields) < 22 {
		return 0, false
	}
	starttime, err := strconv.ParseUint(fields[21], 10, 64)
	if err != nil {
		return 0, false
	}
	return starttime, true
}

func parseProcStatFields(pid int) ([]string, bool) {
	if pid <= 0 {
		return nil, false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, false
	}
	text := string(data)
	closeIdx := strings.LastIndex(text, ")")
	if closeIdx < 0 || closeIdx+2 >= len(text) {
		return nil, false
	}
	rest := strings.TrimSpace(text[closeIdx+2:])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil, false
	}
	// Reconstruct full field list with pid and comm for stable indexing.
	commStart := strings.Index(text, "(")
	if commStart < 0 {
		return nil, false
	}
	pidField := strings.TrimSpace(text[:commStart])
	commField := strings.TrimSpace(text[commStart : closeIdx+1])
	return append([]string{pidField, commField}, fields...), true
}
