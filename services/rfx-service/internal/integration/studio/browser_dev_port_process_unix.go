//go:build unix

package studio

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func killProcessTreePID(pid int) error {
	if pid <= 0 {
		return nil
	}
	return exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
}

func processStarttime(pid int) (uint64, error) {
	starttime, ok := readProcStarttime(pid)
	if !ok {
		return 0, fmt.Errorf("read /proc starttime for pid=%d", pid)
	}
	return starttime, nil
}

func listenerPIDsForPort(port string) ([]int, error) {
	out, err := exec.Command("lsof", "-ti", "tcp:"+port).CombinedOutput()
	if err != nil {
		if len(strings.TrimSpace(string(out))) == 0 {
			return nil, nil
		}
		return nil, err
	}
	pids := make([]int, 0, len(strings.Fields(string(out))))
	for _, pidStr := range strings.Fields(string(out)) {
		pid, convErr := strconv.Atoi(pidStr)
		if convErr != nil || pid <= 0 {
			return nil, fmt.Errorf("invalid listener pid %q on port %s", pidStr, port)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func verifyListenerGroupMembership(listenerPID, expectedPGID int) (bool, string) {
	if listenerPID <= 0 {
		return false, "invalid listener pid"
	}
	if expectedPGID <= 1 {
		return false, "invalid expected pgid"
	}
	livePGID, err := processGroupID(listenerPID)
	if err != nil {
		return false, fmt.Sprintf("read listener pgid pid=%d: %v", listenerPID, err)
	}
	if livePGID != expectedPGID {
		return false, fmt.Sprintf("listener pgid mismatch listener=%d expected=%d live=%d", listenerPID, expectedPGID, livePGID)
	}
	return true, "listener in recorded task-owned pgid"
}

func processGroupHasMembers(pgid int) bool {
	if pgid <= 1 {
		return false
	}
	out, err := exec.Command("ps", "-o", "pid=", "-g", strconv.Itoa(pgid)).CombinedOutput()
	if err != nil {
		return processStillRunning(pgid)
	}
	return len(strings.Fields(string(out))) > 0
}

func waitForPortFreeBounded(port string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !devPortInUse(port) {
			time.Sleep(200 * time.Millisecond)
			return !devPortInUse(port)
		}
		time.Sleep(nuxtGroupPollEvery)
	}
	return !devPortInUse(port)
}

func shutdownTaskOwnedProcessGroup(t *testing.T, record *nuxtProcessLaunchRecord, port string) error {
	t.Helper()
	if err := record.validateForShutdown(port); err != nil {
		return err
	}
	if !devPortInUse(port) {
		return nil
	}
	listeners, err := listenerPIDsForPort(port)
	if err != nil {
		return err
	}
	if len(listeners) == 0 {
		return nil
	}
	if matched, matchErr := launcherIdentityMatches(record); !matched {
		return matchErr
	}
	for _, listenerPID := range listeners {
		ok, reason := verifyListenerGroupMembership(listenerPID, record.PGID)
		if !ok {
			return fmt.Errorf("UNKNOWN_PORT_OWNER_BLOCKED=YES dev port %s pid=%d reason=%s", port, listenerPID, reason)
		}
	}
	if err := signalProcessGroup(record.PGID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("sigterm task-owned pgid=%d port=%s: %w", record.PGID, port, err)
	}
	if waitForPortFreeBounded(port, nuxtGroupTermWait) {
		return nil
	}
	listeners, err = listenerPIDsForPort(port)
	if err != nil {
		return err
	}
	if len(listeners) == 0 && !devPortInUse(port) {
		return nil
	}
	for _, listenerPID := range listeners {
		ok, reason := verifyListenerGroupMembership(listenerPID, record.PGID)
		if !ok {
			return fmt.Errorf("UNKNOWN_PORT_OWNER_BLOCKED=YES before sigkill dev port %s pid=%d reason=%s", port, listenerPID, reason)
		}
	}
	if err := signalProcessGroup(record.PGID, syscall.SIGKILL); err != nil {
		return fmt.Errorf("sigkill task-owned pgid=%d port=%s: %w", record.PGID, port, err)
	}
	if !waitForPortFreeBounded(port, nuxtGroupKillWait) {
		return fmt.Errorf("dev port %s still in use after task-owned group shutdown pgid=%d", port, record.PGID)
	}
	if processGroupHasMembers(record.PGID) {
		return fmt.Errorf("task-owned pgid=%d still has members after group shutdown", record.PGID)
	}
	return nil
}
