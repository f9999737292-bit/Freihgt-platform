package bintranse2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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
	output, err := runSession(t, filepath.Join(t.TempDir(), "session", "stand.env"), filepath.Join(t.TempDir(), "docker-args"), nil, "--sha", "abc")
	if err == nil || !strings.Contains(output, "40-character") {
		t.Fatalf("short sha result %v\n%s", err, output)
	}
}

func TestStandSessionRunnerTrapRunsOnTerm(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
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

func TestStandSessionRunnerRejectsEnvSymlink(t *testing.T) {
	dir := t.TempDir()
	foreign := filepath.Join(dir, "foreign")
	original := []byte("foreign-bytes-v1\n")
	if err := os.WriteFile(foreign, original, 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(foreign)
	if err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(dir, "stand.env")
	if err := os.Symlink(foreign, envFile); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(dir, "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA)
	if err == nil || !strings.Contains(output, "symbolic link") {
		t.Fatalf("symlink env result %v\n%s", err, output)
	}
	assertSessionSecretHidden(t, output)
	assertBytesAndMode(t, foreign, original, before.Mode())
	link, err := os.Lstat(envFile)
	if err != nil || link.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("env symlink was replaced: %v", err)
	}
	assertNoComposeLaunch(t, record)
}

func TestStandSessionRunnerRejectsRepoEnvPath(t *testing.T) {
	envFile := filepath.Join(standRepoRoot(t), "bintrans-e2e-stand-env-rejected.env")
	t.Cleanup(func() { _ = os.Remove(envFile) })
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA)
	if err == nil || !strings.Contains(output, "inside the repository") {
		t.Fatalf("repo env result %v\n%s", err, output)
	}
	assertSessionSecretHidden(t, output)
	if _, statErr := os.Lstat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("repo path created an env file: %v", statErr)
	}
	assertNoComposeLaunch(t, record)
}

func TestStandSessionRunnerRejectsExistingEnvFile(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
	original := []byte("ORIGINAL-ENV-BYTES\n")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, original, 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(envFile)
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA)
	if err == nil || !strings.Contains(output, "already exists") {
		t.Fatalf("existing env result %v\n%s", err, output)
	}
	assertSessionSecretHidden(t, output)
	assertBytesAndMode(t, envFile, original, before.Mode())
	assertNoComposeLaunch(t, record)
}

func TestStandSessionRunnerRejectsParentSymlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := []byte("marker-bytes\n")
	markerPath := filepath.Join(real, "marker")
	if err := os.WriteFile(markerPath, marker, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(link, "stand.env")
	record := filepath.Join(root, "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA)
	if err == nil || !strings.Contains(output, "symbolic link") {
		t.Fatalf("parent symlink result %v\n%s", err, output)
	}
	assertSessionSecretHidden(t, output)
	got, err := os.ReadFile(markerPath)
	if err != nil || !bytes.Equal(got, marker) {
		t.Fatalf("parent target changed: %v %q", err, got)
	}
	if _, statErr := os.Lstat(filepath.Join(real, "stand.env")); !os.IsNotExist(statErr) {
		t.Fatalf("file was created through the parent symlink: %v", statErr)
	}
	assertNoComposeLaunch(t, record)
}

func TestStandSessionRunnerRejectsUnsafeParentPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this host does not store unix directory modes")
	}
	parent := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("directory mode %o is not 0755", info.Mode().Perm())
	}
	envFile := filepath.Join(parent, "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA)
	if err == nil || !strings.Contains(output, "permissions are not 0700") {
		t.Fatalf("unsafe directory result %v\n%s", err, output)
	}
	assertSessionSecretHidden(t, output)
	if _, statErr := os.Lstat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe directory created an env file: %v", statErr)
	}
	assertNoComposeLaunch(t, record)
}

func TestStandSessionRunnerCreatesPrivateEnv0600(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "priv")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	chmodPrivate(t, parent)
	envFile := filepath.Join(parent, "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	modeCh := make(chan os.FileMode, 1)
	go func() {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			info, err := os.Lstat(envFile)
			if err == nil && info.Mode().IsRegular() {
				modeCh <- info.Mode().Perm()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	output, err := runSession(t, envFile, record, map[string]string{"DOCKER_CONFIG_SLEEP": "2"}, "--sha", approvedSessionSHA)
	if err != nil {
		t.Fatalf("private env create failed: %v\n%s", err, output)
	}
	select {
	case mode := <-modeCh:
		if runtime.GOOS != "windows" && mode != 0o600 {
			t.Fatalf("env mode %o", mode)
		}
	case <-time.After(time.Second):
		t.Fatal("private env file was not observed")
	}
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(parent)
		if statErr != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("private directory mode %v", statErr)
		}
		if !strings.Contains(output, "ENV_MODE=0600") {
			t.Fatalf("creator did not report mode 0600\n%s", output)
		}
	}
	assertSessionSecretHidden(t, output)
	assertSameProcessCleanup(t, output)
	if !strings.Contains(output, "SESSION_RESULT=CHECK_ONLY_PASS") || !strings.Contains(output, "EPHEMERAL_ENV=REMOVED") {
		t.Fatalf("private env result %s", output)
	}
	args := readStandFile(t, record)
	if strings.Contains(args, " up ") || strings.Contains(args, " down ") {
		t.Fatalf("private env create invoked up or down\n%s", args)
	}
	if _, statErr := os.Lstat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("private env file remained: %v", statErr)
	}
}

func TestStandSessionRunnerCreateErrorDoesNotLaunchCompose(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(parent, "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runSession(t, envFile, record, nil, "--sha", approvedSessionSHA)
	if err == nil {
		t.Fatalf("create error was accepted\n%s", output)
	}
	assertSessionSecretHidden(t, output)
	if strings.Contains(output, "SESSION_RESULT=CHECK_ONLY_PASS") || strings.Contains(output, " up ") {
		t.Fatalf("create error continued\n%s", output)
	}
	assertNoComposeLaunch(t, record)
}

func TestStandSessionCleanupKeepsReplacedEnvFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "priv")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	chmodPrivate(t, parent)
	envFile := filepath.Join(parent, "stand.env")
	script := `
. scripts/ops/bintrans_e2e_stand/session-common.sh
python=$(stand_python)
native=$(to_native_path "$ENV_FILE")
root=$(to_native_path "$PWD")
printf '%s\n' 'SECRET-TOKEN-DO-NOT-PRINT' | "$python" scripts/ops/bintrans_e2e_stand/private-env.py create "$native" "$root"
`
	output, err := runRepoBash(t, t.TempDir(), map[string]string{"ENV_FILE": toBashPath(envFile)}, script)
	if err != nil {
		t.Fatalf("private env create failed: %v\n%s", err, output)
	}
	if strings.Contains(output, "SECRET-TOKEN-DO-NOT-PRINT") {
		t.Fatalf("creator printed the secret\n%s", output)
	}
	dev := sessionField(t, output, "ENV_DEV")
	ino := sessionField(t, output, "ENV_INO")
	if dev == "" || ino == "" {
		t.Fatalf("identity %s", output)
	}
	if err := os.Remove(envFile); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(parent, "foreign")
	foreignBytes := []byte("foreign-bytes-v1\n")
	if err := os.WriteFile(foreign, foreignBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, envFile); err != nil {
		t.Fatal(err)
	}
	cleanup := "bash scripts/ops/bintrans_e2e_stand/session-cleanup.sh --env-file \"$ENV_FILE\" --resources-started no --env-dev " + dev + " --env-ino " + ino + "\n"
	output, err = runRepoBash(t, t.TempDir(), map[string]string{"ENV_FILE": toBashPath(envFile)}, cleanup)
	if err == nil || !strings.Contains(output, "CLEANUP_FAILED") || strings.Contains(output, "EPHEMERAL_ENV=REMOVED") {
		t.Fatalf("replaced env cleanup %v\n%s", err, output)
	}
	if strings.Contains(output, "SECRET-TOKEN-DO-NOT-PRINT") {
		t.Fatalf("cleanup printed the secret\n%s", output)
	}
	got, err := os.ReadFile(foreign)
	if err != nil || !bytes.Equal(got, foreignBytes) {
		t.Fatalf("foreign bytes changed: %v %q", err, got)
	}
	link, err := os.Lstat(envFile)
	if err != nil || link.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("replacement symlink was removed: %v", err)
	}
}

func TestStandSessionRunnerSigintStopsUpBeforeDown(t *testing.T) {
	assertInterruptedUp(t, "INT", "pid", 130, false)
}

func TestStandSessionRunnerSigtermStopsUpBeforeDown(t *testing.T) {
	assertInterruptedUp(t, "TERM", "pid", 143, false)
}

func TestStandSessionRunnerGroupSigtermStopsUpBeforeDown(t *testing.T) {
	assertInterruptedUp(t, "TERM", "group", 143, false)
}

func TestStandSessionRunnerSignalKeepsEnvWhenDownFails(t *testing.T) {
	assertInterruptedUp(t, "TERM", "pid", 1, true)
}

func assertInterruptedUp(t *testing.T, signal, scope string, want int, downFails bool) {
	t.Helper()
	envFile := filepath.Join(t.TempDir(), "session", "stand.env")
	record := filepath.Join(t.TempDir(), "docker-args")
	logFile := filepath.Join(t.TempDir(), "session.log")
	marker := filepath.Join(t.TempDir(), "marker")
	lock := filepath.Join(t.TempDir(), "up.lock")
	extra := map[string]string{
		"DOCKER_UP_SLEEP": "25",
		"DOCKER_UP_LOCK":  toBashPath(lock),
	}
	if downFails {
		extra["DOCKER_DOWN_RC"] = "1"
	}
	output, err := runSessionUpSignal(t, envFile, record, logFile, marker, signal, scope, extra)
	if err != nil {
		t.Fatalf("signal harness failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "SIGNAL_STATUS="+strconv.Itoa(want)) {
		t.Fatalf("signal status %s", output)
	}
	assertSessionSecretHidden(t, output)
	assertSameProcessCleanup(t, output)
	if !strings.Contains(output, "CHILD_GROUP=ISOLATED") || !strings.Contains(output, "BYSTANDER=ALIVE") || !strings.Contains(output, "CHILD=DEAD") || !strings.Contains(output, "SLEEPER=DEAD") {
		t.Fatalf("signal isolation result %s", output)
	}
	sessionPGID := sessionField(t, output, "SESSION_PGID")
	childPGID := sessionField(t, output, "CHILD_PGID")
	if sessionPGID == "" || sessionPGID == childPGID {
		t.Fatalf("child group %q is not isolated from session %q", childPGID, sessionPGID)
	}
	args := readStandFile(t, record)
	if strings.Contains(args, "UP_FINISHED") || strings.Contains(args, "UP_STILL_RUNNING") || strings.Contains(args, "prune") {
		t.Fatalf("up was still running at cleanup\n%s", args)
	}
	if strings.Index(args, " down ") < strings.Index(args, " up ") || !strings.Contains(args, "ENV_STILL_PRESENT") {
		t.Fatalf("scoped down did not follow the stopped up\n%s", args)
	}
	assertPinnedDockerHost(t, args)
	if downFails {
		if !strings.Contains(output, "CLEANUP_FAILED") || strings.Contains(output, "EPHEMERAL_ENV=REMOVED") {
			t.Fatalf("failed signal cleanup %s", output)
		}
		if _, statErr := os.Stat(envFile); statErr != nil {
			t.Fatalf("failed signal cleanup removed the env file: %v", statErr)
		}
		return
	}
	if strings.Contains(output, "CLEANUP_FAILED") || !strings.Contains(output, "EPHEMERAL_ENV=REMOVED") {
		t.Fatalf("signal cleanup %s", output)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatalf("signal cleanup left the env file: %v", statErr)
	}
}

func chmodPrivate(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
}

func assertBytesAndMode(t *testing.T, path string, original []byte, mode os.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("file bytes changed: %v %q", err, got)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode() != mode {
		t.Fatalf("file mode changed: %v", err)
	}
}

func assertNoComposeLaunch(t *testing.T, record string) {
	t.Helper()
	if _, err := os.Stat(record); err != nil {
		return
	}
	args := readStandFile(t, record)
	if strings.Contains(args, " up ") || strings.Contains(args, " config") {
		t.Fatalf("compose was launched\n%s", args)
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
		"Bash cannot trap SIGKILL",
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
	if strings.Contains(script, "ephemeral-env.sh\" >") || strings.Contains(script, "chmod 0600") {
		t.Fatal("session runner creates the env file by shell redirection")
	}
	creator := readStandFile(t, filepath.Join(standRepoRoot(t), "scripts", "ops", "bintrans_e2e_stand", "private-env.py"))
	for _, token := range []string{"O_CREAT", "O_EXCL", "O_NOFOLLOW"} {
		if !strings.Contains(creator, token) {
			t.Fatalf("private env creator missing %s", token)
		}
	}
	if !strings.Contains(body, "O_CREAT | O_EXCL | O_NOFOLLOW") || !strings.Contains(body, "does not run the EXIT trap") {
		t.Fatal("runbook does not describe exclusive env creation and SIGKILL recovery")
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
while ! grep -q ' config' "$DOCKER_RECORD" 2>/dev/null; do
  i=$((i+1))
  if [ "$i" -gt 100 ]; then
    echo "compose config did not start" >&2
    kill -TERM "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    cat "$SESSION_LOG" >&2
    exit 1
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "runner exited before compose config" >&2
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

func runSessionUpSignal(t *testing.T, envFile, record, logFile, marker, signal, scope string, extra map[string]string) (string, error) {
	t.Helper()
	dir := writeSessionFakes(t, extra)
	script := `set +e
trap 'HARNESS_SIGNAL=1' INT TERM
cleanup_bystander() {
  if [ -n "${bystander:-}" ]; then
    kill "$bystander" 2>/dev/null || true
    wait "$bystander" 2>/dev/null || true
  fi
}
trap cleanup_bystander EXIT
set -m
sleep 120 &
bystander=$!
if [ "$SIGNAL_SCOPE" = "group" ]; then
  set +m
fi
bash scripts/ops/bintrans_e2e_stand/run-session.sh --sha ` + approvedSessionSHA + ` --execute --env-file "$ENV_FILE" >"$SESSION_LOG" 2>&1 &
pid=$!
if [ "$SIGNAL_SCOPE" != "group" ]; then
  set +m
fi
if ps -o pid= -p "$pid" >/dev/null 2>&1; then
  runner_pgid=$(ps -o pgid= -p "$pid" | tr -d '[:space:]')
else
  runner_pgid=$(ps -p "$pid" | awk 'NR>1 {print $3; exit}')
fi
i=0
while ! grep -q '^CHILD_GROUP=ISOLATED$' "$SESSION_MARKER" 2>/dev/null || ! grep -q '^UP_SLEEPER=' "$DOCKER_RECORD" 2>/dev/null; do
  i=$((i+1))
  if [ "$i" -gt 200 ]; then
    echo "up did not become interruptible" >&2
    kill -TERM "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    echo "MARKER" >&2
    cat "$SESSION_MARKER" >&2 || true
    echo "LOG" >&2
    cat "$SESSION_LOG" >&2 || true
    kill "$bystander" 2>/dev/null || true
    wait "$bystander" 2>/dev/null || true
    exit 1
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "runner exited before up" >&2
    cat "$SESSION_LOG" >&2 || true
    kill "$bystander" 2>/dev/null || true
    wait "$bystander" 2>/dev/null || true
    exit 1
  fi
  sleep 0.1
done
if [ "$SIGNAL_SCOPE" = "group" ]; then
  if ps -o pid= -p $$ >/dev/null 2>&1; then
    self_pgid=$(ps -o pgid= -p $$ | tr -d '[:space:]')
  else
    self_pgid=$(ps -p $$ | awk 'NR>1 {print $3; exit}')
  fi
  kill -s TERM -- "-$self_pgid" || true
else
  kill -s "$SIGNAL_NAME" -- "-$runner_pgid" || true
fi
status=0
while kill -0 "$pid" 2>/dev/null; do
  wait "$pid"
  status=$?
done
echo "SIGNAL_STATUS=$status"
if kill -0 "$bystander" 2>/dev/null; then
  echo BYSTANDER=ALIVE
else
  echo BYSTANDER=DEAD
fi
kill "$bystander" 2>/dev/null || true
wait "$bystander" 2>/dev/null || true
sleeper=$(awk -F= '$1=="UP_SLEEPER" {print $2}' "$DOCKER_RECORD" | tail -n 1 | tr -d '[:space:]')
if [ -n "$sleeper" ] && kill -0 "$sleeper" 2>/dev/null; then
  echo SLEEPER=ALIVE
else
  echo SLEEPER=DEAD
fi
child=$(awk -F= '$1=="CHILD_PID" {print $2}' "$SESSION_MARKER" | tail -n 1 | tr -d '[:space:]')
if [ -n "$child" ] && kill -0 "$child" 2>/dev/null; then
  echo CHILD=ALIVE
else
  echo CHILD=DEAD
fi
cat "$SESSION_LOG"
`
	env := map[string]string{
		"ENV_FILE":       toBashPath(envFile),
		"DOCKER_RECORD":  toBashPath(record),
		"SESSION_LOG":    toBashPath(logFile),
		"SESSION_MARKER": toBashPath(marker),
		"DOCKER_HOST":    "",
		"DOCKER_CONTEXT": "",
		"SIGNAL_NAME":    signal,
		"SIGNAL_SCOPE":   scope,
		"GIT_HEAD":       approvedSessionSHA,
		"GIT_STATUS":     "",
	}
	for key, value := range extra {
		env[key] = value
	}
	return runRepoBash(t, dir, env, script)
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
