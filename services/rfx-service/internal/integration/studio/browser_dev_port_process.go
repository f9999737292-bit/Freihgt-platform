package studio

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

func processStillRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		script := fmt.Sprintf(
			`$p=Get-Process -Id %d -ErrorAction SilentlyContinue; if ($null -eq $p) { exit 1 } else { exit 0 }`,
			pid,
		)
		return exec.Command("powershell", "-NoProfile", "-Command", script).Run() == nil
	}
	err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run()
	return err == nil
}

func waitForProcessExitBounded(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processStillRunning(pid) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return !processStillRunning(pid)
}
