package studio

import (
	"testing"
	"time"
)

func TestWaitForProcessExitBounded_alreadyExited(t *testing.T) {
	if !waitForProcessExitBounded(99999999, 500*time.Millisecond) {
		t.Fatalf("expected non-existent pid to be treated as exited")
	}
}

func TestProcessStillRunning_nonExistentPid(t *testing.T) {
	if processStillRunning(99999999) {
		t.Fatalf("expected non-existent pid to not be running")
	}
}
