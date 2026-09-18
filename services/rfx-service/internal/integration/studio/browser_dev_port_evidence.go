package studio

import (
	"bytes"
	"fmt"
	"strings"
)

// ProcessEvidence captures normalized process identity used for port ownership policy.
type ProcessEvidence struct {
	PID         int
	Executable  string
	Argv        string
	CWD         string
	CWDVerified bool
	Port        string
	ParentPID   int
}

func (e ProcessEvidence) String() string {
	return fmt.Sprintf(
		"pid=%d exe=%q argv=%q cwd=%q cwdVerified=%v port=%s ppid=%d",
		e.PID,
		e.Executable,
		sanitizeCommandLineForLog(e.Argv, 120),
		e.CWD,
		e.CWDVerified,
		e.Port,
		e.ParentPID,
	)
}

func readProcCmdlineFromBytes(data []byte) string {
	parts := bytes.Split(data, []byte{0})
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		args = append(args, string(part))
	}
	return strings.Join(args, " ")
}

func processEvidenceFromPortProc(port string, proc portProcessInfo) ProcessEvidence {
	cwd, cwdVerified := verifiedProcessCwd(proc)
	return ProcessEvidence{
		PID:         proc.PID,
		Executable:  proc.ProcessName,
		Argv:        proc.CommandLine,
		CWD:         cwd,
		CWDVerified: cwdVerified,
		Port:        port,
		ParentPID:   proc.ParentPID,
	}
}
