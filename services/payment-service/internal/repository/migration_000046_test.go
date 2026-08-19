package repository

import (
	"path/filepath"
	"strings"
	"testing"
)

func readMigration(t *testing.T, name string) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name),
		filepath.Join("..", "..", "..", "infrastructure", "migrations", name),
	}
	for _, candidate := range candidates {
		content, err := readFileIfExists(candidate)
		if err == nil {
			return content
		}
	}
	t.Fatalf("migration file not found: %s", name)
	return ""
}

func readFileIfExists(path string) (string, error) {
	b, err := readFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func TestMigration000046UpCreatesPaymentOutbox(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000046_payment_paid_projection_outbox_v1.9.2A.up.sql"))
	if !strings.Contains(up, "create table billing.payment_outbox") {
		t.Fatal("up migration must create billing.payment_outbox")
	}
	if !strings.Contains(up, "uq_payment_outbox_tenant_event_aggregate") {
		t.Fatal("up migration must define unique (tenant_id, event_type, aggregate_id)")
	}
	if !strings.Contains(up, "idx_payment_outbox_pending") {
		t.Fatal("up migration must create pending index")
	}
	if !strings.Contains(up, "chk_payment_outbox_status") {
		t.Fatal("up migration must define status check constraint")
	}
	if !strings.Contains(up, "chk_payment_outbox_attempts") {
		t.Fatal("up migration must define attempts check constraint")
	}
	if strings.Contains(up, "voided_by") || strings.Contains(up, "voided_at") {
		t.Fatal("000046 must not add void metadata")
	}
}

func TestMigration000046DownDropsPaymentOutbox(t *testing.T) {
	t.Parallel()
	down := strings.ToLower(readMigration(t, "000046_payment_paid_projection_outbox_v1.9.2A.down.sql"))
	if !strings.Contains(down, "drop table") || !strings.Contains(down, "billing.payment_outbox") {
		t.Fatal("down migration must drop billing.payment_outbox")
	}
}
