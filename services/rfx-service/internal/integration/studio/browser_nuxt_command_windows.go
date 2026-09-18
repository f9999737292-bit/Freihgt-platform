//go:build windows

package studio

import (
	"os/exec"
	"syscall"
)

func applyTaskOwnedNuxtProcessGroup(cmd *exec.Cmd) {
	_ = cmd
}

func currentProcessGroupID() (int, error) {
	return 0, nil
}

func processGroupID(pid int) (int, error) {
	_ = pid
	return 0, nil
}

func signalProcessGroup(pgid int, sig syscall.Signal) error {
	_ = pgid
	_ = sig
	return nil
}
