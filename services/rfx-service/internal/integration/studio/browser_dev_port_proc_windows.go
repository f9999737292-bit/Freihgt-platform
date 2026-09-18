//go:build windows

package studio

func readProcCmdline(pid int) string {
	_ = pid
	return ""
}

func readProcExe(pid int) string {
	_ = pid
	return ""
}

func readProcParentPID(pid int) int {
	_ = pid
	return 0
}
