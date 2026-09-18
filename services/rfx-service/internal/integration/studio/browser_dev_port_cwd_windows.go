//go:build windows

package studio

func lookupProcessCwd(pid int) string {
	// Native Windows does not expose a stable cwd lookup without kernel reads.
	// Fail closed for relative argv; absolute command-line paths remain verifiable.
	return ""
}
