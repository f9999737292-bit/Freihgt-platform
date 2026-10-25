package bintranse2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

func TestStandSessionRunnerCheckOnlyRemovesEnvWithoutUp(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA)
	if err != nil {
		t.Fatalf("check-only failed: %v\n%s", err, output)
	}
	assertSessionSecretHidden(t, output)
	assertSameProcessCleanup(t, output)
	if !strings.Contains(output, "SESSION_MODE=CHECK_ONLY") || !strings.Contains(output, "SESSION_RESULT=CHECK_ONLY_PASS") {
		t.Fatalf("check-only result %s", output)
	}
	if !strings.Contains(output, "DOCKER_HOST=unix:///var/run/docker.sock") || !strings.Contains(output, "DOCKER_CONTEXT_UNSET=YES") {
		t.Fatalf("endpoint was not pinned\n%s", output)
	}
	args := readStandFile(t, record)
	if strings.Contains(args, " up ") || strings.Contains(args, " down ") || strings.Contains(args, "prune") {
		t.Fatalf("check-only invoked up, down, or prune\n%s", args)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("check-only left the env file: %v", statErr)
	}
	observeSessionRecord(t, record, envFile)
}

func TestStandSessionRunnerStopsWhenPreflightFails(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, map[string]string{"UNAME_MACHINE": "aarch64"}, "--sha", approvedSessionSHA)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("preflight failure exit %v\n%s", err, output)
	}
	if strings.Contains(output, "SESSION_RESULT=CHECK_ONLY_PASS") || strings.Contains(output, " up ") {
		t.Fatalf("preflight failure continued\n%s", output)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("preflight failure created an env file: %v", statErr)
	}
	if _, statErr := os.Stat(record); statErr == nil && strings.Contains(readStandFile(t, record), " up ") {
		t.Fatal("preflight failure invoked up")
	}
}

func TestStandSessionRunnerRemovesEnvWhenProjectGuardFails(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	count := filepath.Join(t.TempDir(), "guard-count")
	output, err := runSession(t, envFile, record, map[string]string{
		"SESSION_GUARD_FAIL":  "1",
		"SESSION_GUARD_COUNT": toBashPath(count),
	}, "--sha", approvedSessionSHA)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("guard failure exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "project guard failed") || !strings.Contains(output, "EPHEMERAL_ENV=REMOVED") {
		t.Fatalf("guard failure result %s", output)
	}
	assertSessionSecretHidden(t, output)
	if strings.Contains(output, " up ") {
		t.Fatalf("guard failure invoked up\n%s", output)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("guard failure left the env file: %v", statErr)
	}
}

func TestStandSessionRunnerRemovesEnvWhenComposeConfigFails(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, map[string]string{"DOCKER_CONFIG_RC": "1"}, "--sha", approvedSessionSHA)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("config failure exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "compose config failed") || !strings.Contains(output, "EPHEMERAL_ENV=REMOVED") {
		t.Fatalf("config failure result %s", output)
	}
	assertSessionSecretHidden(t, output)
	args := readStandFile(t, record)
	if strings.Contains(args, " up ") || strings.Contains(args, " down ") {
		t.Fatalf("config failure invoked up or down\n%s", args)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("config failure left the env file: %v", statErr)
	}
}

func TestStandSessionRunnerExecuteCleansUpAfterSuccessfulUp(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA, "--execute")
	if err != nil {
		t.Fatalf("execute failed: %v\n%s", err, output)
	}
	assertSessionSecretHidden(t, output)
	assertSameProcessCleanup(t, output)
	args := readStandFile(t, record)
	upAt := strings.Index(args, " up ")
	downAt := strings.Index(args, " down ")
	if upAt < 0 || downAt < upAt || !strings.Contains(args, "ENV_STILL_PRESENT") {
		t.Fatalf("execute cleanup order\n%s", args)
	}
	if strings.Contains(args, "prune") || strings.Contains(output, "CLEANUP_FAILED") {
		t.Fatalf("successful execute cleanup\n%s\n%s", output, args)
	}
	assertPinnedDockerHost(t, args)
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("execute left the env file: %v", statErr)
	}
	observeSessionRecord(t, record, envFile)
}

func TestStandSessionRunnerExecutePreservesUpStatus(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, map[string]string{"DOCKER_UP_RC": "7"}, "--sha", approvedSessionSHA, "--execute")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("failed up exit %v\n%s", err, output)
	}
	args := readStandFile(t, record)
	if strings.Index(args, " down ") < strings.Index(args, " up ") || !strings.Contains(args, "ENV_STILL_PRESENT") {
		t.Fatalf("failed up did not run scoped down\n%s", args)
	}
	assertSessionSecretHidden(t, output)
	assertPinnedDockerHost(t, args)
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("failed up left the env file: %v", statErr)
	}
}

func TestStandSessionRunnerKeepsEnvWhenDownFails(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, map[string]string{"DOCKER_DOWN_RC": "1"}, "--sha", approvedSessionSHA, "--execute")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("failed down exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "CLEANUP_FAILED") || strings.Contains(output, "EPHEMERAL_ENV=REMOVED") {
		t.Fatalf("failed down result %s", output)
	}
	args := readStandFile(t, record)
	if !strings.Contains(args, "ENV_STILL_PRESENT") || strings.Contains(args, "prune") {
		t.Fatalf("failed down record %s", args)
	}
	assertSessionSecretHidden(t, output)
	if _, statErr := os.Stat(envFile); statErr != nil {
		t.Fatalf("failed down removed the env file: %v", statErr)
	}
}

func TestStandSessionRunnerRejectsRemoteContextBeforeUp(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, map[string]string{
		"DOCKER_CONTEXT":          "remote",
		"DOCKER_CONTEXT_ENDPOINT": "tcp://203.0.113.10:2375",
	}, "--sha", approvedSessionSHA, "--execute")
	if err == nil {
		t.Fatalf("remote context was accepted\n%s", output)
	}
	if strings.Contains(output, " up ") || strings.Contains(output, "SESSION_MODE=EXECUTE") && strings.Contains(output, "docker compose up") {
		t.Fatalf("remote context reached up\n%s", output)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		args := readStandFile(t, record)
		if strings.Contains(args, " up ") || strings.Contains(args, "tcp://") && strings.Contains(args, " up ") {
			t.Fatalf("remote context invoked up\n%s", args)
		}
		if strings.Contains(args, "tcp://") && strings.Contains(args, "compose config") {
			t.Fatalf("remote context reached compose config\n%s", args)
		}
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("remote context created an env file: %v", statErr)
	}
}

func TestStandSessionRunnerRejectsShortSha(t *testing.T) {
	output, err := runSession(t, filepath.Join(t.TempDir(), "stand.env"), filepath.Join(t.TempDir(), "docker-args"), nil, "--sha", "abc")
	if err == nil || !strings.Contains(output, "40-character") {
		t.Fatalf("short sha result %v\n%s", err, output)
	}
}

func TestStandSessionRunnerTrapRunsOnTerm(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	logFile := filepath.Join(t.TempDir(), "session.log")
	output, err := runSessionSignal(t, envFile, record, logFile)
	if err != nil {
		t.Fatalf("signal harness failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "SIGNAL_STATUS=143") {
		t.Fatalf("signal status %s", output)
	}
	assertSameProcessCleanup(t, output)
	assertSessionSecretHidden(t, output)
	if strings.Contains(output, " up ") || strings.Contains(output, " down ") {
		t.Fatalf("signal path invoked up or down\n%s", output)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("signal path left the env file: %v", statErr)
	}
}

func TestStandExecutionRunbookUsesOneSessionProcess(t *testing.T) {
	body := readStandFile(t, filepath.Join(standRepoRoot(t), "docs", "architecture", "e2e-isolated-stand", "DEDICATED_HOST_EXECUTION_RUNBOOK_0_1F.md"))
	check := "bash scripts/ops/bintrans_e2e_stand/run-session.sh --sha <CONTROLLER_APPROVED_SHA>"
	execute := check + " --execute"
	markers := []string{
		"git clone https://github.com/f9999737292-bit/Freihgt-platform.git",
		"cd Freihgt-platform",
		"git checkout <CONTROLLER_APPROVED_SHA>",
		"its own controller approval",
		check,
		execute,
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
	script := readStandFile(t, filepath.Join(standRepoRoot(t), "scripts", "ops", "bintrans_e2e_stand", "run-session.sh"))
	if strings.Contains(script, "prune") {
		t.Fatal("session runner contains prune")
	}
	for _, token := range []string{"trap cleanup EXIT", "trap 'exit 130' INT", "trap 'exit 143' TERM", "unset DOCKER_CONTEXT", "export DOCKER_HOST=\"$verified_endpoint\""} {
		if !strings.Contains(script, token) {
			t.Fatalf("session runner missing %q", token)
		}
	}
	started := strings.Index(script, "resources_started=yes")
	up := strings.Index(script, "up --build --abort-on-container-exit --exit-code-from runner")
	if started < 0 || up < started {
		t.Fatal("resources_started is not set immediately before up")
	}
}

const approvedSessionSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const sessionSecret = "knownsessionsecretknownsession"

func assertSessionSecretHidden(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, sessionSecret) || strings.Contains(output, "BINTRANS_E2E_STAND_POSTGRES_PASSWORD=") {
		t.Fatalf("session output leaked ephemeral secret material\n%s", output)
	}
}

func assertSameProcessCleanup(t *testing.T, output string) {
	t.Helper()
	pid := sessionField(t, output, "SESSION_PID")
	cleanup := sessionField(t, output, "SESSION_CLEANUP_PID")
	if pid == "" || pid != cleanup {
		t.Fatalf("cleanup pid %q is not session pid %q\n%s", cleanup, pid, output)
	}
}

func assertPinnedDockerHost(t *testing.T, args string) {
	t.Helper()
	const endpoint = "unix:///var/run/docker.sock"
	sawPinned := false
	for _, line := range strings.Split(args, "\n") {
		if strings.Contains(line, "tcp://") || strings.Contains(line, "ssh://") {
			t.Fatalf("session reached a remote endpoint\n%s", line)
		}
		if strings.Contains(line, " up ") || strings.Contains(line, " down ") || strings.Contains(line, " config") {
			if !strings.Contains(line, "ENV_DOCKER_HOST="+endpoint) || !strings.Contains(line, "ENV_DOCKER_CONTEXT=<unset>") {
				t.Fatalf("docker command left the pinned endpoint\n%s", line)
			}
			sawPinned = true
		}
	}
	if !sawPinned {
		t.Fatal("no pinned docker command was recorded")
	}
}

func sessionField(t *testing.T, output, key string) string {
	t.Helper()
	prefix := key + "="
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), prefix))
		}
	}
	return ""
}

func observeSessionRecord(t *testing.T, record, envFile string) {
	t.Helper()
	script := "printf '%s\\n' OBSERVE_ONLY; cat \"$1\"; if [ -f \"$2\" ]; then echo ENV_STILL_THERE; else echo ENV_ABSENT; fi\n"
	cmd := exec.Command(standBash(t), "-s", "--", toBashPath(record), toBashPath(envFile))
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("observer failed: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "OBSERVE_ONLY") || !strings.Contains(text, "ENV_ABSENT") {
		t.Fatalf("observer result %s", text)
	}
	if strings.Contains(script, "compose up") || strings.Contains(script, "down.sh") || strings.Contains(script, "trap ") {
		t.Fatal("observer script performs session work")
	}
}

func runSession(t *testing.T, envFile, record string, extra map[string]string, args ...string) (string, error) {
	t.Helper()
	dir := writeSessionFakes(t, extra)
	command := "bash scripts/ops/bintrans_e2e_stand/run-session.sh --env-file \"$ENV_FILE\""
	for _, arg := range args {
		command += " " + arg
	}
	command += "\n"
	env := map[string]string{
		"ENV_FILE":       toBashPath(envFile),
		"DOCKER_RECORD":  toBashPath(record),
		"DOCKER_HOST":    "",
		"DOCKER_CONTEXT": "",
		"GIT_HEAD":       approvedSessionSHA,
		"GIT_STATUS":     "",
	}
	for key, value := range extra {
		env[key] = value
	}
	return runRepoBash(t, dir, env, command)
}

func runSessionSignal(t *testing.T, envFile, record, logFile string) (string, error) {
	t.Helper()
	dir := writeSessionFakes(t, map[string]string{"DOCKER_CONFIG_SLEEP": "15"})
	script := `bash scripts/ops/bintrans_e2e_stand/run-session.sh --sha ` + approvedSessionSHA + ` --env-file "$ENV_FILE" >"$SESSION_LOG" 2>&1 &
pid=$!
i=0
while [ ! -f "$ENV_FILE" ]; do
  i=$((i+1))
  if [ "$i" -gt 100 ]; then
    echo "env file was not created" >&2
    kill -TERM "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    cat "$SESSION_LOG" >&2
    exit 1
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "runner exited before the env file existed" >&2
    cat "$SESSION_LOG" >&2
    exit 1
  fi
  sleep 0.1
done
kill -TERM "$pid"
set +e
wait "$pid"
status=$?
set -e
echo "SIGNAL_STATUS=$status"
cat "$SESSION_LOG"
`
	return runRepoBash(t, dir, map[string]string{
		"ENV_FILE":            toBashPath(envFile),
		"DOCKER_RECORD":       toBashPath(record),
		"SESSION_LOG":         toBashPath(logFile),
		"DOCKER_HOST":         "",
		"DOCKER_CONTEXT":      "",
		"DOCKER_CONFIG_SLEEP": "15",
		"GIT_HEAD":            approvedSessionSHA,
		"GIT_STATUS":          "",
	}, script)
}

func writeSessionFakes(t *testing.T, extra map[string]string) string {
	t.Helper()
	dir := writeHostPreflightFakes(t)
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
	writeFakeCommand(t, dir, "openssl", "#!/bin/sh\nprintf '%s\\n' \""+sessionSecret+"\"\n")
	if extra != nil && extra["SESSION_GUARD_FAIL"] == "1" {
		writeFakeCommand(t, dir, "bash", `#!/bin/sh
real=/usr/bin/bash
if [ "${1:-}" = "-s" ] || [ "${1:-}" = "-c" ] || [ "${1:-}" = "-lc" ]; then
  exec "$real" "$@"
fi
for arg in "$@"; do
  case "$arg" in
    *project-guard.sh*)
      n=0
      if [ -n "${SESSION_GUARD_COUNT:-}" ] && [ -f "$SESSION_GUARD_COUNT" ]; then
        n=$(cat "$SESSION_GUARD_COUNT")
      fi
      n=$((n+1))
      if [ -n "${SESSION_GUARD_COUNT:-}" ]; then
        printf '%s\n' "$n" > "$SESSION_GUARD_COUNT"
      fi
      if [ "$n" -ge 2 ]; then
        echo "project guard failed" >&2
        exit 1
      fi
      ;;
  esac
done
exec "$real" "$@"
`)
	}
	return dir
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
