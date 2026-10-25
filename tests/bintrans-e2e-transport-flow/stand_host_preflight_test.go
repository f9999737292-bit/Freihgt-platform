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
		"DOCKER_DAEMON=AVAILABLE",
		"DOCKER_COMPOSE_PLUGIN=PRESENT",
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
	for _, token := range []string{" up", "pull", "build", "prune", "systemctl"} {
		if strings.Contains(args, token) {
			t.Fatalf("preflight invoked %q\n%s", token, args)
		}
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
	assertHostPreflightFail(t, map[string]string{"DOCKER_ENDPOINT": "tcp://0.0.0.0:2375"}, "unix socket")
}

func TestStandHostPreflightRejectsPublishedStandPorts(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{
		"DOCKER_PS": "bintrans-e2e-stand-v01a|0.0.0.0:5432->5432/tcp",
	}, "host ports")
}

func TestStandHostPreflightRejectsMissingComposePlugin(t *testing.T) {
	assertHostPreflightFail(t, map[string]string{"DOCKER_COMPOSE": "missing"}, "compose plugin")
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
	overrides := map[string]string{"DOCKER_RECORD": toBashPath(record)}
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
	writeFakeCommand(t, dir, "docker", `#!/bin/sh
printf '%s\n' "$*" >> "$DOCKER_RECORD"
if [ "${DOCKER_DAEMON:-up}" != "up" ]; then
  echo "docker daemon is not running" >&2
  exit 1
fi
cmd=$1
shift || true
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
    printf '%s\n' "${DOCKER_ENDPOINT:-unix:///var/run/docker.sock}"
    ;;
  network)
    if [ -n "${DOCKER_NETWORKS:-}" ]; then
      printf '%s\n' $DOCKER_NETWORKS
    fi
    ;;
  volume)
    if [ -n "${DOCKER_VOLUMES:-}" ]; then
      printf '%s\n' $DOCKER_VOLUMES
    fi
    ;;
  ps)
    if [ -n "${DOCKER_PS:-}" ]; then
      printf '%s\n' "$DOCKER_PS"
    fi
    ;;
  compose)
    sub=${1:-}
    if [ "$sub" = "version" ]; then
      if [ "${DOCKER_COMPOSE:-present}" != "present" ]; then
        echo "compose plugin missing" >&2
        exit 1
      fi
      echo "Docker Compose version v5.1.4"
      exit 0
    fi
    if [ "$sub" = "ls" ]; then
      if [ -n "${DOCKER_PROJECTS:-}" ]; then
        printf '%s\n' $DOCKER_PROJECTS
      fi
      exit 0
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
