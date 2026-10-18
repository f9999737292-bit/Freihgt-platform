package repository

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestMigration000099Contract(t *testing.T) {
	dir := locateRepoMigrations(t)
	up := string(mustReadMigration(t, filepath.Join(dir, "000099_edo_pod_delivery_action_evidence_binding_v0_1b.up.sql")))
	down := string(mustReadMigration(t, filepath.Join(dir, "000099_edo_pod_delivery_action_evidence_binding_v0_1b.down.sql")))
	lower := strings.ToLower(up)
	if strings.Contains(lower, "references transport.") {
		t.Fatal("migration creates a foreign key into transport")
	}
	for _, needle := range []string{
		"documents.pod_delivery_action_evidence",
		"documents.pod_delivery_action_upload_intent",
		"document_tenant_id = operating_tenant_id",
		"document_id uuid NOT NULL UNIQUE",
		"UNIQUE (document_tenant_id, idempotency_key)",
		"UNIQUE (document_tenant_id, execution_action_id, idempotency_key)",
		"pod_delivery_action_evidence_subject_idx",
	} {
		if !strings.Contains(up, needle) {
			t.Fatalf("up migration missing %s", needle)
		}
	}
	fn := up[strings.Index(up, "CREATE FUNCTION documents.reject_pod_delivery_action_evidence_identity_update()"):]
	fn = fn[:strings.Index(fn, "CREATE TRIGGER")]
	if strings.Contains(fn, "IS DISTINCT FROM") || !strings.Contains(fn, "RAISE EXCEPTION 'POD_ACTION_EVIDENCE_IMMUTABLE'") {
		t.Fatal("evidence rows must reject every update")
	}
	if strings.Contains(strings.ToLower(up), "insert into documents.pod_delivery_action_evidence") {
		t.Fatal("migration backfills evidence")
	}
	repo := string(mustReadMigration(t, "pod_action_evidence_repository.go"))
	if strings.Contains(repo, "transport.") || strings.Contains(repo, "'local'") {
		t.Fatal("action repository reads transport or hardcodes local storage")
	}
	if !strings.Contains(down, "DROP TABLE IF EXISTS documents.pod_delivery_action_evidence") {
		t.Fatal("down migration does not drop the evidence table")
	}
	assertMigrationChain(t, dir, 100)
}

func assertMigrationChain(t *testing.T, dir string, head int) {
	t.Helper()
	ups, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	downs, err := filepath.Glob(filepath.Join(dir, "*.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	upNums := migrationNumbers(t, ups)
	downNums := migrationNumbers(t, downs)
	if len(upNums) != head || len(downNums) != head {
		t.Fatalf("migration count up=%d down=%d head=%d", len(upNums), len(downNums), head)
	}
	for i := 1; i <= head; i++ {
		if upNums[i-1] != i || downNums[i-1] != i {
			t.Fatalf("chain break at %d up=%d down=%d", i, upNums[i-1], downNums[i-1])
		}
	}
}

func migrationNumbers(t *testing.T, files []string) []int {
	t.Helper()
	sort.Strings(files)
	out := make([]int, 0, len(files))
	for _, file := range files {
		base := filepath.Base(file)
		n, err := strconv.Atoi(base[:6])
		if err != nil {
			t.Fatalf("bad migration name %s", base)
		}
		out = append(out, n)
	}
	return out
}

func locateRepoMigrations(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	} {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	t.Fatal("migrations directory not found")
	return ""
}

func mustReadMigration(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
