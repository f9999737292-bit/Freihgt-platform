package bintranse2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStandHTTPListenHostIsLoopbackOnlyInStand(t *testing.T) {
	if standHTTPListenHost("stand") != "127.0.0.1" {
		t.Fatalf("stand host %q", standHTTPListenHost("stand"))
	}
	for _, mode := range []string{"", "ci", "prod"} {
		if host := standHTTPListenHost(mode); host != "" {
			t.Fatalf("mode %q host %q", mode, host)
		}
	}
}

func TestStandDownPassesEnvFile(t *testing.T) {
	const secret = "super-secret-value"
	envFile := writeStandEnvFile(t, secret)
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runStandBash(t, map[string]string{
		"PATH":          fakeDockerPath(t, record),
		"DOCKER_RECORD": record,
	}, "down.sh", "--env-file", envFile)
	if err != nil {
		t.Fatalf("down.sh failed: %v\n%s", err, output)
	}
	args := readStandFile(t, record)
	for _, token := range []string{"--env-file", "stand.env", "-p", "bintrans-e2e-stand-v0.1a", "down", "-v", "--remove-orphans"} {
		if !strings.Contains(args, token) {
			t.Fatalf("docker args missing %q\n%s", token, args)
		}
	}
	if strings.Contains(args, secret) || strings.Contains(output, secret) {
		t.Fatal("cleanup output included ephemeral secret material")
	}
	if strings.Contains(args, "prune") || strings.Contains(args, "bintrans-ct-staging") {
		t.Fatalf("docker args left the stand project: %s", args)
	}
}

func TestStandDownRejectsForeignProjectBeforeDocker(t *testing.T) {
	const secret = "super-secret-value"
	envFile := writeStandEnvFile(t, secret)
	record := filepath.Join(t.TempDir(), "docker-args")
	for _, project := range []string{"bintrans-ct-staging", "freight-platform", "bintrans-e2e-stand-v0.1b"} {
		output, err := runStandBash(t, map[string]string{
			"PATH":                 fakeDockerPath(t, record),
			"DOCKER_RECORD":        record,
			"COMPOSE_PROJECT_NAME": project,
		}, "down.sh", "--env-file", envFile)
		if err == nil {
			t.Fatalf("project %s was accepted\n%s", project, output)
		}
		if _, statErr := os.Stat(record); statErr == nil {
			t.Fatalf("project %s invoked docker\n%s", project, readStandFile(t, record))
		}
		if strings.Contains(output, secret) {
			t.Fatalf("project %s error leaked credential material", project)
		}
	}
}

func TestStandDownRejectsProjectOverrideBeforeDocker(t *testing.T) {
	envFile := writeStandEnvFile(t, "super-secret-value")
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runStandBash(t, map[string]string{
		"PATH":          fakeDockerPath(t, record),
		"DOCKER_RECORD": record,
	}, "down.sh", "--env-file", envFile, "-p", "bintrans-ct-staging")
	if err == nil {
		t.Fatalf("project override was accepted\n%s", output)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatal("project override invoked docker")
	}
}

func TestStandDownMissingEnvFileBeforeDocker(t *testing.T) {
	record := filepath.Join(t.TempDir(), "docker-args")
	output, err := runStandBash(t, map[string]string{
		"PATH":          fakeDockerPath(t, record),
		"DOCKER_RECORD": record,
	}, "down.sh", "--env-file", filepath.Join(t.TempDir(), "missing.env"))
	if err == nil {
		t.Fatalf("missing env file was accepted\n%s", output)
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatal("missing env file invoked docker")
	}
}

func TestStandProjectGuardRejectsForeignExplicitProject(t *testing.T) {
	output, err := runStandBash(t, nil, "project-guard.sh", "--project", "bintrans-ct-staging")
	if err == nil {
		t.Fatalf("foreign project was accepted\n%s", output)
	}
	if strings.Contains(output, "docker compose") {
		t.Fatalf("guard attempted compose: %s", output)
	}
}

func writeStandEnvFile(t *testing.T, secret string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stand.env")
	body := "BINTRANS_E2E_STAND_POSTGRES_USER=stand_user\nBINTRANS_E2E_STAND_POSTGRES_PASSWORD=" + secret + "\nBINTRANS_E2E_STAND_POSTGRES_DB=bintrans_e2e_stand\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeDockerPath(t *testing.T, record string) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "docker")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$DOCKER_RECORD\"\nexit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = record
	path := dir
	if current := os.Getenv("PATH"); current != "" {
		path = dir + string(os.PathListSeparator) + current
	}
	return path
}

func runStandBash(t *testing.T, overrides map[string]string, script string, args ...string) (string, error) {
	t.Helper()
	bash := standBash(t)
	scriptPath := filepath.Join(standRepoRoot(t), "scripts", "ops", "bintrans_e2e_stand", script)
	cmdArgs := append([]string{toBashPath(scriptPath)}, args...)
	for i, arg := range cmdArgs {
		if i == 0 {
			continue
		}
		if strings.Contains(arg, `\`) || (len(arg) >= 2 && arg[1] == ':') {
			cmdArgs[i] = toBashPath(arg)
		}
	}
	if overrides != nil {
		if record, ok := overrides["DOCKER_RECORD"]; ok {
			copied := make(map[string]string, len(overrides))
			for key, value := range overrides {
				copied[key] = value
			}
			copied["DOCKER_RECORD"] = toBashPath(record)
			overrides = copied
		}
	}
	cmd := exec.Command(bash, cmdArgs...)
	cmd.Env = standScriptEnv(overrides)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func standBash(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		for _, candidate := range []string{
			`C:\Program Files\Git\bin\bash.exe`,
			`C:\Program Files\Git\usr\bin\bash.exe`,
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	if path, err := exec.LookPath("bash"); err == nil {
		return path
	}
	t.Fatal("bash is required to test stand scripts")
	return ""
}

func standScriptEnv(overrides map[string]string) []string {
	skip := map[string]struct{}{"COMPOSE_PROJECT_NAME": {}}
	for key := range overrides {
		skip[key] = struct{}{}
	}
	out := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, found := skip[key]; found {
			continue
		}
		out = append(out, entry)
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

func toBashPath(path string) string {
	if runtime.GOOS != "windows" {
		return filepath.ToSlash(path)
	}
	slash := filepath.ToSlash(path)
	if len(slash) >= 2 && slash[1] == ':' {
		return "/" + strings.ToLower(slash[:1]) + slash[2:]
	}
	return slash
}
