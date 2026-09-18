package studio

import (
	"os/exec"
	"runtime"
	"strings"
)

func devPortInUse(port string) bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "netstat -ano -p tcp").CombinedOutput()
		if err != nil {
			return true
		}
		needle := ":" + port
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(strings.ToUpper(line), "LISTENING") && strings.Contains(line, needle) {
				return true
			}
		}
		return false
	}
	out, err := exec.Command("lsof", "-ti", "tcp:"+port).CombinedOutput()
	if err != nil {
		return len(strings.TrimSpace(string(out))) > 0
	}
	return len(strings.Fields(string(out))) > 0
}
