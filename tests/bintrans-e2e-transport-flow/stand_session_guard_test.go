package bintranse2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestStandCheckoutGuardRejectsWrongSha(t *testing.T) {
	output, err := runCheckoutGuard(t, "deadbeef", "", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err == nil || !strings.Contains(output, "approved sha") {
		t.Fatalf("wrong sha result %v\n%s", err, output)
	}
}

func TestStandCheckoutGuardRejectsDirtyWorktree(t *testing.T) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	output, err := runCheckoutGuard(t, sha, " M docs/unrelated.md", sha)
	if err == nil || !strings.Contains(output, "not clean") {
		t.Fatalf("dirty worktree result %v\n%s", err, output)
	}
}

func TestStandCheckoutGuardAcceptsApprovedSha(t *testing.T) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	output, err := runCheckoutGuard(t, sha, "", sha)
	if err != nil || !strings.Contains(output, "CHECKOUT_GUARD=PASS") {
		t.Fatalf("approved checkout result %v\n%s", err, output)
	}
}

func TestStandSessionTrapRemovesEnvOnError(t *testing.T) {
	const secret = "stand-secret-value"
	envFile := writeStandEnvFile(t, secret)
	output, err := runSessionTrap(t, envFile, "no", "", "", 9)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 9 {
		t.Fatalf("error-path exit %v\n%s", err, output)
	}
	if strings.Contains(output, secret) {
		t.Fatal("trap output included the ephemeral secret")
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("env file remained after error exit: %v", statErr)
	}
}

func TestStandSessionTrapCleansResourcesBeforeDeletingEnv(t *testing.T) {
	const secret = "stand-secret-value"
	envFile := writeStandEnvFile(t, secret)
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSessionTrap(t, envFile, "yes", cleanupDockerFake(t, 0), record, 0)
	if err != nil {
		t.Fatalf("cleanup trap failed: %v\n%s", err, output)
	}
	args := readStandFile(t, record)
	if !strings.Contains(args, "ENV_STILL_PRESENT") || !strings.Contains(args, "down") {
		t.Fatalf("down.sh did not see the env file\n%s", args)
	}
	if strings.Contains(args, "prune") || strings.Contains(output, secret) {
		t.Fatalf("cleanup leaked scope or secret\n%s\n%s", args, output)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("env file remained after successful cleanup: %v", statErr)
	}
}

func TestStandSessionTrapKeepsEnvWhenCleanupFails(t *testing.T) {
	const secret = "stand-secret-value"
	envFile := writeStandEnvFile(t, secret)
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSessionTrap(t, envFile, "yes", cleanupDockerFake(t, 1), record, 0)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("failed cleanup exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "CLEANUP_FAILED") {
		t.Fatalf("failed cleanup result %s", output)
	}
	args := readStandFile(t, record)
	if !strings.Contains(args, "ENV_STILL_PRESENT") || strings.Contains(args, "prune") {
		t.Fatalf("failed cleanup record %s", args)
	}
	if strings.Contains(output, secret) {
		t.Fatal("failed cleanup printed the ephemeral secret")
	}
	if _, statErr := os.Stat(envFile); statErr != nil {
		t.Fatalf("env file was removed after CLEANUP_FAILED: %v", statErr)
	}
}

func TestStandExecutionRunbookOrdersCheckoutAndCleanup(t *testing.T) {
	body := readStandFile(t, filepath.Join(standRepoRoot(t), "docs", "architecture", "e2e-isolated-stand", "DEDICATED_HOST_EXECUTION_RUNBOOK_0_1F.md"))
	markers := []string{
		"git clone https://github.com/f9999737292-bit/Freihgt-platform.git",
		"cd Freihgt-platform",
		"git checkout <CONTROLLER_APPROVED_SHA>",
		"bash scripts/ops/bintrans_e2e_stand/checkout-guard.sh --sha <CONTROLLER_APPROVED_SHA>",
		"bash scripts/ops/bintrans_e2e_stand/host-preflight.sh",
		"BEGIN SESSION_TRAP",
		"session-cleanup.sh",
		"chmod 0600",
		"project-guard.sh --project bintrans-e2e-stand-v01a",
		"config --quiet",
		"resources_started=yes",
		"CLEANUP_FAILED",
	}
	last := -1
	for _, marker := range markers {
		at := strings.Index(body, marker)
		if at < 0 || at < last {
			t.Fatalf("runbook order missing or regressing at %q", marker)
		}
		last = at
	}
	if strings.Contains(body, "git checkout 0005a659429404fe5a30425e4bbdbba899b6daa0") {
		t.Fatal("runbook checks out the pre-preflight baseline")
	}
	script := readStandFile(t, filepath.Join(standRepoRoot(t), "scripts", "ops", "bintrans_e2e_stand", "session-cleanup.sh"))
	if strings.Contains(script, "prune") || !strings.Contains(script, "CLEANUP_FAILED") {
		t.Fatal("session cleanup script lost its failure contract")
	}
}

func runCheckoutGuard(t *testing.T, head, status, sha string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	writeFakeCommand(t, dir, "git", `#!/bin/sh
if [ "$1" = "rev-parse" ]; then
  printf '%s\n' "${GIT_HEAD:?}"
  exit 0
fi
if [ "$1" = "status" ]; then
  printf '%s' "${GIT_STATUS-}"
  exit 0
fi
echo "unsupported git command" >&2
exit 1
`)
	script := "bash scripts/ops/bintrans_e2e_stand/checkout-guard.sh --sha " + sha + "\n"
	return runRepoBash(t, dir, map[string]string{
		"GIT_HEAD":   head,
		"GIT_STATUS": status,
	}, script)
}

func runSessionTrap(t *testing.T, envFile, started, fakeDir, record string, exitCode int) (string, error) {
	t.Helper()
	if fakeDir == "" {
		fakeDir = t.TempDir()
	}
	body := readStandFile(t, filepath.Join(standRepoRoot(t), "docs", "architecture", "e2e-isolated-stand", "DEDICATED_HOST_EXECUTION_RUNBOOK_0_1F.md"))
	start := strings.Index(body, "# BEGIN SESSION_TRAP\n")
	end := strings.Index(body, "# END SESSION_TRAP")
	if start < 0 || end < start {
		t.Fatal("session trap markers are missing")
	}
	extra := map[string]string{
		"ENV_FILE":          toBashPath(envFile),
		"RESOURCES_STARTED": started,
	}
	if record != "" {
		extra["DOCKER_RECORD"] = toBashPath(record)
	}
	script := body[start:end] + "\nexit " + strconv.Itoa(exitCode) + "\n"
	return runRepoBash(t, fakeDir, extra, script)
}

func cleanupDockerFake(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	writeFakeCommand(t, dir, "docker", `#!/bin/sh
printf '%s\n' "$*" >> "$DOCKER_RECORD"
prev=""
envfile=""
for arg in "$@"; do
  if [ "$prev" = "--env-file" ]; then
    envfile=$arg
  fi
  prev=$arg
done
if [ -n "$envfile" ] && [ -f "$envfile" ]; then
  echo ENV_STILL_PRESENT >> "$DOCKER_RECORD"
fi
exit `+strconv.Itoa(exitCode)+`
`)
	return dir
}

func runRepoBash(t *testing.T, fakeDir string, extra map[string]string, script string) (string, error) {
	t.Helper()
	cmd := exec.Command(standBash(t), "-c", `fake=$1
case "$fake" in
  [A-Za-z]:*) fake=$(cygpath -u "$fake") ;;
esac
export PATH="$fake:$PATH"
shift
exec bash -s -- "$@"`, "session", fakeDir)
	cmd.Dir = standRepoRoot(t)
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = standScriptEnv(extra)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
