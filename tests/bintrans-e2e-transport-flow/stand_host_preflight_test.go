package bintranse2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandHostPreflightPassesHealthyHost(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, nil)
	if err != nil {
		t.Fatalf("preflight failed: %v\n%s", err, output)
	}
	for _, token := range []string{
		"PREFLIGHT_RESULT=PASS",
		"ARCH=x86_64",
		"KERNEL=Linux",
		"VCPU=4",
		"RAM_THRESHOLDS=PASS",
		"DOCKER_DAEMON=AVAILABLE",
		"DOCKER_COMPOSE_PLUGIN=PRESENT",
		"DOCKER_ENDPOINT=unix://",
		"DOCKER_ENDPOINT_SCHEME=unix",
		"COMPOSE_FILE_PROJECT=bintrans-e2e-stand-v01a",
		"COMPOSE_HOST_PORTS=ABSENT",
		"PROJECT_GUARD=PASS",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("preflight missing %q\n%s", token, output)
		}
	}
	args := readStandFile(t, record)
	for _, token := range []string{" up", "pull", "build", "prune", "systemctl", "compose ls -a --format"} {
		if strings.Contains(args, token) {
			t.Fatalf("preflight invoked %q\n%s", token, args)
		}
	}
	if !strings.Contains(args, "compose ls -a -q") || !strings.Contains(args, "--host") {
		t.Fatalf("preflight did not pin the daemon or list projects quietly\n%s", args)
	}
}

func TestStandHostPreflightRejectsArmArchitecture(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"UNAME_MACHINE": "aarch64"}, "architecture")
}

func TestStandHostPreflightRejectsLowCpu(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"NPROC_COUNT": "2"}, "vCPU")
}

func TestStandHostPreflightRejectsLowRam(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, map[string]string{
		"FREE_TOTAL_BYTES": "8589934592",
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("low RAM exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "PREFLIGHT_RESULT=FAIL") || !strings.Contains(output, "total RAM") {
		t.Fatalf("low RAM result %s", output)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatalf("low RAM invoked docker\n%s", readStandFile(t, record))
	}
}

func TestStandHostPreflightBlocksWhenDockerDown(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, map[string]string{
		"DOCKER_DAEMON": "down",
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("missing daemon exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "PREFLIGHT_RESULT=BLOCKED") {
		t.Fatalf("missing daemon result %s", output)
	}
	args := readStandFile(t, record)
	for _, token := range []string{" up", "pull", "build", "prune", "systemctl"} {
		if strings.Contains(args, token) {
			t.Fatalf("blocked preflight invoked %q\n%s", token, args)
		}
	}
}

func TestStandHostPreflightRejectsLowDockerDisk(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DF_AVAILABLE_BYTES": "10737418240"}, "free space")
}

func TestStandHostPreflightRejectsExistingStandProject(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_PROJECTS": "bintrans-e2e-stand-v01a"}, "conflicts")
}

func TestStandHostPreflightRejectsStagingProject(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_PROJECTS": "bintrans-ct-staging"}, "conflicts")
}

func TestStandHostPreflightRejectsSharedNetwork(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_NETWORKS": "freight-platform-network"}, "freight-platform-network")
}

func TestStandHostPreflightRejectsSharedVolume(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_VOLUMES": "docker-compose_freight_postgres_data"}, "shared postgres volume")
}

func TestStandHostPreflightRejectsPublishedDockerAPI(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_ENDPOINT": "tcp://0.0.0.0:2375"}, "remote")
}

func TestStandHostPreflightRejectsRemoteDockerHostBeforeDaemon(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, map[string]string{
		"DOCKER_HOST":     "tcp://203.0.113.10:2375",
		"DOCKER_CONTEXT":  "default",
		"DOCKER_ENDPOINT": "unix:///var/run/docker.sock",
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("remote DOCKER_HOST exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "PREFLIGHT_RESULT=FAIL") || !strings.Contains(output, "DOCKER_HOST") {
		t.Fatalf("remote DOCKER_HOST result %s", output)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatalf("remote DOCKER_HOST invoked docker\n%s", readStandFile(t, record))
	}
}

func TestStandHostPreflightRejectsRemoteContextOverLocalDockerHost(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, map[string]string{
		"DOCKER_HOST":             "unix:///var/run/docker.sock",
		"DOCKER_CONTEXT":          "remote",
		"DOCKER_CONTEXT_ENDPOINT": "ssh://builder.example/var/run/docker.sock",
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("remote context exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "PREFLIGHT_RESULT=FAIL") || !strings.Contains(output, "remote") {
		t.Fatalf("remote context result %s", output)
	}
	if args := readStandFile(t, record); strings.Contains(args, "info") {
		t.Fatalf("remote context contacted the daemon\n%s", args)
	}
}

func TestStandHostPreflightRejectsComposeLsFailure(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_COMPOSE_LS_RC": "1"}, "docker compose ls failed")
}

func TestStandHostPreflightRejectsNetworkLsFailure(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_NETWORK_LS_RC": "1"}, "docker network ls failed")
}

func TestStandHostPreflightRejectsVolumeLsFailure(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_VOLUME_LS_RC": "1"}, "docker volume ls failed")
}

func TestStandHostPreflightRejectsPsFailure(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_PS_RC": "1"}, "docker ps failed")
}

func TestStandHostPreflightRejectsOneMiBAvailableRam(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, map[string]string{
		"FREE_TOTAL_BYTES": "16000000000",
		"FREE_AVAIL_BYTES": "1048576",
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("low available RAM exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "PREFLIGHT_RESULT=FAIL") || !strings.Contains(output, "available RAM") {
		t.Fatalf("low available RAM result %s", output)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatalf("low available RAM invoked docker\n%s", readStandFile(t, record))
	}
}

func TestStandHostPreflightRejectsAvailableRamBelowSixGiB(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{
		"FREE_TOTAL_BYTES": "16000000000",
		"FREE_AVAIL_BYTES": "6442450943",
	}, "available RAM")
}

func TestStandHostPreflightAcceptsExactRamThresholds(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, map[string]string{
		"FREE_TOTAL_BYTES": "16000000000",
		"FREE_AVAIL_BYTES": "6442450944",
	})
	if err != nil {
		t.Fatalf("exact RAM thresholds failed: %v\n%s", err, output)
	}
	for _, token := range []string{
		"MIN_TOTAL_RAM_BYTES=16000000000",
		"MIN_AVAILABLE_RAM_BYTES=6442450944",
		"MIN_DOCKER_DISK_FREE_BYTES=53687091200",
		"RAM_THRESHOLDS=PASS",
		"PREFLIGHT_RESULT=PASS",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("exact RAM result missing %q\n%s", token, output)
		}
	}
}

func TestStandHostPreflightRejectsTotalRamBelowSixteenDecimalGB(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, map[string]string{
		"FREE_TOTAL_BYTES": "15999999999",
		"FREE_AVAIL_BYTES": "6442450944",
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("low total RAM exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "PREFLIGHT_RESULT=FAIL") || !strings.Contains(output, "total RAM") {
		t.Fatalf("low total RAM result %s", output)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatalf("low total RAM invoked docker\n%s", readStandFile(t, record))
	}
}

func TestStandHostPreflightRejectsPublishedStandPorts(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{
		"DOCKER_PS": "bintrans-e2e-stand-v01a|0.0.0.0:5432->5432/tcp",
	}, "host ports")
}

func TestStandHostPreflightRejectsMissingComposePlugin(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_COMPOSE": "missing"}, "docker compose version failed")
}

func TestStandHostPreflightUsesControllerUnits(t *testing.T) {
	text := readStandFile(t, filepath.Join(standRepoRoot(t), "scripts", "ops", "bintrans_e2e_stand", "host-preflight.sh"))
	for _, token := range []string{"16000000000", "6442450944", "53687091200", "compose ls -a -q", "env test -S"} {
		if !strings.Contains(text, token) {
			t.Fatalf("preflight source missing %q", token)
		}
	}
	for _, token := range []string{"16 * 1024 * 1024 * 1024", "17179869184", "$(docker"} {
		if strings.Contains(text, token) {
			t.Fatalf("preflight source still contains %q", token)
		}
	}
	for _, doc := range []string{"DEDICATED_HOST_SECURITY_RUNBOOK_0_1F.md", "DEDICATED_HOST_EXECUTION_RUNBOOK_0_1F.md"} {
		body := readStandFile(t, filepath.Join(standRepoRoot(t), "docs", "architecture", "e2e-isolated-stand", doc))
		for _, token := range []string{"16000000000", "16 decimal GB", "6442450944", "6 GiB", "53687091200", "50 GiB"} {
			if !strings.Contains(body, token) {
				t.Fatalf("%s missing %q", doc, token)
			}
		}
	}
}

func TestStandHostPreflightSourceDoesNotMutateHost(t *testing.T) {
	text := readStandFile(t, filepath.Join(standRepoRoot(t), "scripts", "ops", "bintrans_e2e_stand", "host-preflight.sh"))
	for _, token := range []string{"systemctl", "service start", "compose up", "docker pull", "docker build", "prune", "wsl --shutdown"} {
		if strings.Contains(text, token) {
			t.Fatalf("preflight source contains %q", token)
		}
	}
	if !strings.Contains(text, "bintrans-e2e-stand-v01a") || !strings.Contains(text, "PREFLIGHT_RESULT=BLOCKED") {
		t.Fatal("preflight source lost the project id or blocked result")
	}
}

func assertHostPreflightFail(t *testing.T, extra map[string]string, needle string) {
	t.Helper()
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runHostPreflight(t, record, extra)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("preflight exit %v\n%s", err, output)
	}
	if !strings.Contains(output, "PREFLIGHT_RESULT=FAIL") || !strings.Contains(output, needle) {
		t.Fatalf("preflight output missing fail evidence %q\n%s", needle, output)
	}
}

func runHostPreflight(t *testing.T, record string, extra map[string]string) (string, error) {
	t.Helper()
	fakeDir := writeHostPreflightFakes(t)
	scriptPath := toBashPath(filepath.Join(standRepoRoot(t), "scripts", "ops", "bintrans_e2e_stand", "host-preflight.sh"))
	// Git Bash prepends /usr/bin when it converts a Windows PATH, which hides
	// stand-ins for uname, nproc, and df. Prepend after that conversion.
	cmd := exec.Command(standBash(t), "-c", `fake=$1
case "$fake" in
  [A-Za-z]:*) fake=$(cygpath -u "$fake") ;;
esac
export PATH="$fake:$PATH"
shift
exec bash "$@"`, "preflight", fakeDir, scriptPath)
	overrides := map[string]string{
		"DOCKER_RECORD":  toBashPath(record),
		"DOCKER_HOST":    "",
		"DOCKER_CONTEXT": "",
	}
	for key, value := range extra {
		overrides[key] = value
	}
	cmd.Env = standScriptEnv(overrides)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeHostPreflightFakes(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFakeCommand(t, dir, "uname", `#!/bin/sh
case "$1" in
  -m) printf '%s\n' "${UNAME_MACHINE:-x86_64}" ;;
  -s) printf '%s\n' "${UNAME_KERNEL:-Linux}" ;;
  *) exit 1 ;;
esac
`)
	writeFakeCommand(t, dir, "nproc", "#!/bin/sh\nprintf '%s\\n' \"${NPROC_COUNT:-4}\"\n")
	writeFakeCommand(t, dir, "free", `#!/bin/sh
printf '%s\n' "              total        used        free      shared  buff/cache   available"
printf 'Mem: %s 1 1 0 0 %s\n' "${FREE_TOTAL_BYTES:-17179869184}" "${FREE_AVAIL_BYTES:-8589934592}"
`)
	writeFakeCommand(t, dir, "df", `#!/bin/sh
printf '%s\n' "Filesystem 1024-blocks Used Available Capacity Mounted"
printf 'overlay 1 1 %s 1%% /\n' "${DF_AVAILABLE_BYTES:-64424509440}"
`)
	writeFakeCommand(t, dir, "test", `#!/bin/sh
if [ "$1" = "-S" ]; then
  if [ "${TEST_SOCKET_EXISTS:-yes}" = "yes" ]; then
    exit 0
  fi
  exit 1
fi
exec /usr/bin/test "$@"
`)
	writeFakeCommand(t, dir, "docker", `#!/bin/sh
context_marker=${DOCKER_CONTEXT-<unset>}
printf 'ENV_DOCKER_HOST=%s ENV_DOCKER_CONTEXT=%s ARGS=%s\n' "${DOCKER_HOST-}" "$context_marker" "$*" >> "$DOCKER_RECORD"
while [ "${1:-}" = "--host" ] || [ "${1:-}" = "-H" ]; do
  shift 2
done
cmd=${1:-}
shift || true
if [ "$cmd" != "context" ] && [ "${DOCKER_DAEMON:-up}" != "up" ]; then
  echo "docker daemon is not running" >&2
  exit 1
fi
case "$cmd" in
  info)
    if [ "${1:-}" = "--format" ]; then
      printf '%s\n' "${DOCKER_ROOT:-/var/lib/docker}"
    fi
    ;;
  version)
    printf '%s\n' "${DOCKER_ENGINE_VERSION:-29.5.3}"
    ;;
  context)
    action=${1:-}
    shift || true
    case "$action" in
      show)
        printf '%s\n' "${DOCKER_CURRENT_CONTEXT:-default}"
        ;;
      inspect)
        name=${1:-}
        if [ -n "${DOCKER_CONTEXT_ENDPOINT:-}" ] && [ -n "${DOCKER_CONTEXT:-}" ] && [ "$name" = "$DOCKER_CONTEXT" ]; then
          printf '%s\n' "$DOCKER_CONTEXT_ENDPOINT"
        else
          printf '%s\n' "${DOCKER_ENDPOINT:-unix:///var/run/docker.sock}"
        fi
        ;;
      *)
        echo "unsupported context command" >&2
        exit 1
        ;;
    esac
    ;;
  network)
    if [ "${1:-}" = "ls" ] && [ "${DOCKER_NETWORK_LS_RC:-0}" != "0" ]; then
      echo "network ls failed" >&2
      exit 1
    fi
    if [ -n "${DOCKER_NETWORKS:-}" ]; then
      printf '%s\n' $DOCKER_NETWORKS
    fi
    ;;
  volume)
    if [ "${1:-}" = "ls" ] && [ "${DOCKER_VOLUME_LS_RC:-0}" != "0" ]; then
      echo "volume ls failed" >&2
      exit 1
    fi
    if [ -n "${DOCKER_VOLUMES:-}" ]; then
      printf '%s\n' $DOCKER_VOLUMES
    fi
    ;;
  ps)
    if [ "${DOCKER_PS_RC:-0}" != "0" ]; then
      echo "ps failed" >&2
      exit 1
    fi
    if [ -n "${DOCKER_PS:-}" ]; then
      printf '%s\n' "$DOCKER_PS"
    fi
    ;;
  compose)
    sub=""
    prev=""
    envfile=""
    quiet=0
    for arg in "$@"; do
      if [ "$prev" = "--env-file" ]; then
        envfile=$arg
      fi
      case "$arg" in
        --quiet) quiet=1 ;;
        version|ls|config|up|down) sub=$arg ;;
      esac
      prev=$arg
    done
    if [ "$sub" = "version" ]; then
      if [ "${DOCKER_COMPOSE:-present}" != "present" ]; then
        echo "compose plugin missing" >&2
        exit 1
      fi
      echo "Docker Compose version v5.1.4"
      exit 0
    fi
    if [ "$sub" = "ls" ]; then
      if [ "${DOCKER_COMPOSE_LS_RC:-0}" != "0" ]; then
        echo "compose ls failed" >&2
        exit 1
      fi
      if [ -n "${DOCKER_PROJECTS:-}" ]; then
        printf '%s\n' $DOCKER_PROJECTS
      fi
      exit 0
    fi
    if [ "$sub" = "config" ]; then
      if [ -n "${DOCKER_CONFIG_SLEEP:-}" ]; then
        sleep "$DOCKER_CONFIG_SLEEP"
      fi
      if [ "${DOCKER_CONFIG_RC:-0}" != "0" ]; then
        echo "config failed" >&2
        exit 1
      fi
      if [ "$quiet" -eq 1 ]; then
        exit 0
      fi
      printf '%s\n' "name: bintrans-e2e-stand-v01a"
      if [ -n "$envfile" ] && [ -f "$envfile" ]; then
        grep '^BINTRANS_E2E_STAND_POSTGRES_PASSWORD=' "$envfile" || true
      fi
      exit 0
    fi
    if [ "$sub" = "up" ]; then
      exit "${DOCKER_UP_RC:-0}"
    fi
    if [ "$sub" = "down" ]; then
      if [ -n "$envfile" ] && [ -f "$envfile" ]; then
        echo ENV_STILL_PRESENT >> "$DOCKER_RECORD"
      fi
      exit "${DOCKER_DOWN_RC:-0}"
    fi
    echo "unsupported compose command: $*" >> "$DOCKER_RECORD"
    exit 1
    ;;
  *)
    echo "unsupported docker command: $cmd" >> "$DOCKER_RECORD"
    exit 1
    ;;
esac
`)
	return dir
}

func writeFakeCommand(t *testing.T, dir, name, body string) {
	t.Helper()
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
