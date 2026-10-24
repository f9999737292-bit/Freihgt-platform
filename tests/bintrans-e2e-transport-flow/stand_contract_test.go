package bintranse2e

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestStandMigrationHead000105(t *testing.T) {
	dir := filepath.Join(standRepoRoot(t), "infrastructure", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) != standSchemaVersion {
		t.Fatalf("up files=%d, want %d", len(files), standSchemaVersion)
	}
	for i, file := range files {
		base := filepath.Base(file)
		want := fmt.Sprintf("%06d_", i+1)
		if !strings.HasPrefix(base, want) {
			t.Fatalf("file %s, want prefix %s", base, want)
		}
		if strings.Contains(base, "enforcement_gate") {
			t.Fatalf("migration tree contains enforcement gate file %s", base)
		}
	}
	last := filepath.Base(files[len(files)-1])
	if !strings.HasPrefix(last, "000105_") {
		t.Fatalf("migration head %s", last)
	}
}

func TestStandComposeIsolation(t *testing.T) {
	path := filepath.Join(standRepoRoot(t), "infrastructure", "docker-compose", "docker-compose.bintrans-e2e-stand-v0.1a.yml")
	text := readStandFile(t, path)
	services := composeServiceNames(text)
	want := []string{"postgres", "redpanda", "topic-init", "migrate", "runner"}
	if strings.Join(services, ",") != strings.Join(want, ",") {
		t.Fatalf("services %v", services)
	}
	required := []string{
		"name: bintrans-e2e-stand-v0.1a",
		"image: mirror.gcr.io/library/postgres:16",
		"image: docker.redpanda.com/redpandadata/redpanda:v24.2.8",
		"image: migrate/migrate:v4.17.1",
		"internal: true",
		"bintrans-e2e-stand-postgres:",
		"bintrans-e2e-stand-documents:",
		"bintrans-e2e-stand-redpanda:",
		"- up",
		"BINTRANS_E2E_STAND_MODE: stand",
		"BINTRANS_E2E_KAFKA_BROKERS: redpanda:9092",
		"@postgres:5432/",
		"TMS_RELATIONSHIP_ACTIVATION_ENABLED: \"false\"",
		"internal://redpanda:9092",
		"localhost:9092",
	}
	for _, token := range required {
		if !strings.Contains(text, token) {
			t.Fatalf("compose missing %q", token)
		}
	}
	forbidden := []string{
		"ports:",
		"container_name:",
		"env_file:",
		"network_mode:",
		"profiles:",
		"external: true",
		"freight_postgres_data",
		"freight-platform-network",
		"freight_postgres",
		"bintrans-ct-staging",
		"staging.env",
		"/protected/",
		"dev_secret_change_me",
		"freight_password",
		"BINTRANS_E2E_KFAKE_ALLOW",
		"tms_activation_enforcement_gate",
		"127.0.0.1",
		"docker-compose.yml",
		"docker-compose.bintrans-ct-staging.yml",
	}
	for _, token := range forbidden {
		if strings.Contains(text, token) {
			t.Fatalf("compose contains %q", token)
		}
	}
	if strings.Count(text, "localhost:9092") != 1 {
		t.Fatalf("localhost:9092 occurrences=%d", strings.Count(text, "localhost:9092"))
	}
}

func TestStandRunnerImageStaysOffline(t *testing.T) {
	root := standRepoRoot(t)
	dockerfile := readStandFile(t, filepath.Join(root, "infrastructure", "docker", "bintrans-e2e-stand", "Dockerfile"))
	entrypoint := readStandFile(t, filepath.Join(root, "infrastructure", "docker", "bintrans-e2e-stand", "runner-entrypoint.sh"))
	for _, token := range []string{
		"ENV GOPROXY=off",
		"ENV GOFLAGS=-mod=readonly",
		"COPY services/shipment-service",
		"COPY services/control-tower-read-model-service",
		"COPY tests/bintrans-e2e-transport-flow",
	} {
		if !strings.Contains(dockerfile, token) {
			t.Fatalf("dockerfile missing %q", token)
		}
	}
	for _, token := range []string{"COPY . ", "staging.env", "/protected/", "BINTRANS_E2E_KFAKE_ALLOW", "EXPOSE "} {
		if strings.Contains(dockerfile, token) {
			t.Fatalf("dockerfile contains %q", token)
		}
	}
	for _, token := range []string{
		"BINTRANS_E2E_KFAKE_ALLOW",
		"BINTRANS_E2E_STAND_MODE:-}\" != \"stand\"",
		"redpanda:9092",
		"TEST_DATABASE_URL",
		"DOCUMENT_STORAGE_ROOT",
	} {
		if !strings.Contains(entrypoint, token) {
			t.Fatalf("entrypoint missing %q", token)
		}
	}
	if strings.Contains(entrypoint, "echo \"$TEST_DATABASE_URL\"") || strings.Contains(entrypoint, "echo $TEST_DATABASE_URL") {
		t.Fatal("entrypoint prints TEST_DATABASE_URL")
	}
}

func TestStandCleanupIsProjectScoped(t *testing.T) {
	root := standRepoRoot(t)
	script := readStandFile(t, filepath.Join(root, "scripts", "ops", "bintrans_e2e_stand", "down.sh"))
	envScript := readStandFile(t, filepath.Join(root, "scripts", "ops", "bintrans_e2e_stand", "ephemeral-env.sh"))
	for _, token := range []string{
		"--env-file",
		"-p \"$project\"",
		"bintrans-e2e-stand-v0.1a",
		"docker-compose.bintrans-e2e-stand-v0.1a.yml",
		"down -v --remove-orphans",
		"project-guard.sh",
	} {
		if !strings.Contains(script, token) {
			t.Fatalf("cleanup missing %q", token)
		}
	}
	for _, token := range []string{
		"volume prune",
		"system prune",
		"network prune",
		"freight_postgres_data",
		"freight-platform-network",
		"bintrans-ct-staging",
		"BINTRANS_E2E_KFAKE_ALLOW",
		"staging.env",
	} {
		if strings.Contains(script, token) || strings.Contains(envScript, token) {
			t.Fatalf("stand script contains %q", token)
		}
	}
	if strings.Contains(envScript, "freight_password") || strings.Contains(envScript, "dev_secret_change_me") {
		t.Fatal("ephemeral env script contains a platform credential")
	}
	guard := readStandFile(t, filepath.Join(root, "scripts", "ops", "bintrans_e2e_stand", "project-guard.sh"))
	if !strings.Contains(guard, "bintrans-e2e-stand-v0.1a") || !strings.Contains(guard, "COMPOSE_PROJECT_NAME") {
		t.Fatal("project guard does not check the stand project")
	}
	for _, token := range []string{"docker compose", "volume prune", "system prune", "bintrans-ct-staging"} {
		if strings.Contains(guard, token) {
			t.Fatalf("project guard contains %q", token)
		}
	}
	if strings.Index(script, "project-guard.sh") > strings.Index(script, "docker compose") {
		t.Fatal("cleanup reaches docker compose before the project guard")
	}
}

func TestStandHarnessRefusesSecondMigrationOwner(t *testing.T) {
	harness := readStandFile(t, filepath.Join(standRepoRoot(t), "tests", "bintrans-e2e-transport-flow", "harness.go"))
	policy := readStandFile(t, filepath.Join(standRepoRoot(t), "tests", "bintrans-e2e-transport-flow", "broker_policy.go"))
	for _, token := range []string{
		"stand mode refuses applyMigrations",
		"stand mode refuses CREATE DATABASE",
		"stand mode refuses kfake.NewCluster",
		"TMS_RELATIONSHIP_ACTIVATION_ENABLED\": \"false\"",
		"decideBroker",
	} {
		if !strings.Contains(harness, token) {
			t.Fatalf("harness missing %q", token)
		}
	}
	if strings.Contains(harness, "tms_activation_enforcement_gate") {
		t.Fatal("harness references the enforcement gate")
	}
	if strings.Contains(policy, "kfake.NewCluster") || strings.Contains(policy, "pkg/kfake") {
		t.Fatal("broker policy starts kfake")
	}
}

func TestStandCIJobKeepsExplicitKfakeAllow(t *testing.T) {
	text := readStandFile(t, filepath.Join(standRepoRoot(t), ".github", "workflows", "ci.yml"))
	start := strings.Index(text, "  bintrans-e2e-transport-flow:")
	end := strings.Index(text, "  rfx-questionnaire-v3-integration:")
	if start < 0 || end < start {
		t.Fatal("ci job markers missing")
	}
	job := text[start:end]
	for _, token := range []string{
		"BINTRANS_E2E_STAND_MODE: ci",
		"BINTRANS_E2E_KFAKE_ALLOW: \"1\"",
		"TestBintransE2ETransportFlow",
		"TestBroker|TestStand",
	} {
		if !strings.Contains(job, token) {
			t.Fatalf("ci job missing %q", token)
		}
	}
}

func TestStandOptInWorkflowIsManual(t *testing.T) {
	text := readStandFile(t, filepath.Join(standRepoRoot(t), ".github", "workflows", "bintrans-e2e-stand-opt-in.yml"))
	if !strings.Contains(text, "workflow_dispatch:") {
		t.Fatal("opt-in workflow is not manual")
	}
	for _, token := range []string{"pull_request", "push:", "${{ secrets"} {
		if strings.Contains(text, token) {
			t.Fatalf("opt-in workflow contains %q", token)
		}
	}
	if strings.Contains(text, "BINTRANS_E2E_KFAKE_ALLOW:") || strings.Contains(text, "BINTRANS_E2E_KFAKE_ALLOW=") {
		t.Fatal("opt-in workflow sets BINTRANS_E2E_KFAKE_ALLOW")
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "--env-file") && (strings.Contains(line, "staging.env") || strings.Contains(line, "/protected/")) {
			t.Fatalf("opt-in workflow loads a protected environment file: %s", strings.TrimSpace(line))
		}
	}
	if !strings.Contains(text, "-p bintrans-e2e-stand-v0.1a") || !strings.Contains(text, "down.sh --env-file") || !strings.Contains(text, "project-guard.sh") {
		t.Fatal("opt-in cleanup is not project scoped")
	}
}

func standRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.work")); err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return root
}

func readStandFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func composeServiceNames(content string) []string {
	lines := strings.Split(content, "\n")
	inServices := false
	var names []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "services:" {
			inServices = true
			continue
		}
		if inServices && line != "" && line[0] != ' ' && line[0] != '#' {
			break
		}
		if !inServices || !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, " ") || !strings.HasSuffix(trimmed, ":") {
			continue
		}
		names = append(names, strings.TrimSuffix(trimmed, ":"))
	}
	return names
}
