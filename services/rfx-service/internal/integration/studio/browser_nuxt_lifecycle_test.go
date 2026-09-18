//go:build integration

package studio

import "testing"

func TestStopNuxtDevProcess_idempotentWithoutCmd(t *testing.T) {
	stopNuxtDevProcess(t, nil, "")
}

func TestReleaseTaskOwnedDevPort_noOpForUnscopedPort(t *testing.T) {
	releaseTaskOwnedDevPort(t, "3020", testWorktreeRoot(t))
}

func TestVerifyDevPortsReleased_skipsUnscopedPorts(t *testing.T) {
	verifyDevPortsReleased(t, "3020")
}
