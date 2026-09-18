//go:build unix

package studio

import (
	"os/exec"
	"syscall"
)

func applyTaskOwnedNuxtProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}

func currentProcessGroupID() (int, error) {
	return syscall.Getpgid(syscall.Getpid())
}

func processGroupID(pid int) (int, error) {
	if pid <= 0 {
		return 0, syscall.ESRCH
	}
	return syscall.Getpgid(pid)
}

func signalProcessGroup(pgid int, sig syscall.Signal) error {
	if pgid <= 1 {
		return syscall.EINVAL
	}
	return syscall.Kill(-pgid, sig)
}
