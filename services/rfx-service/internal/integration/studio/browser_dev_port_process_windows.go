//go:build windows

package studio

import (
	"fmt"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func killProcessTreePID(pid int) error {
	if pid <= 0 {
		return nil
	}
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}

func processStarttime(pid int) (uint64, error) {
	_ = pid
	return 0, fmt.Errorf("process starttime unsupported on windows")
}

func listenerPIDsForPort(port string) ([]int, error) {
	_ = port
	return nil, fmt.Errorf("listener pid lookup unsupported on windows")
}

func verifyListenerGroupMembership(listenerPID, expectedPGID int) (bool, string) {
	_ = listenerPID
	_ = expectedPGID
	return false, "process group membership unsupported on windows"
}

func processGroupHasMembers(pgid int) bool {
	_ = pgid
	return false
}

func waitForPortFreeBounded(port string, timeout time.Duration) bool {
	_ = timeout
	return !devPortInUse(port)
}

func shutdownTaskOwnedProcessGroup(t *testing.T, record *nuxtProcessLaunchRecord, port string) error {
	_ = t
	_ = record
	_ = port
	return fmt.Errorf("process group shutdown unsupported on windows")
}
