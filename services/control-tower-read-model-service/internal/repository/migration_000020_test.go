package repository

import (
	"strings"
	"testing"
)

func TestMigration000020UpCreatesAcknowledgementTable(t *testing.T) {
	t.Parallel()
	up := strings.ToLower(readMigration(t, "000020_create_control_tower_critical_event_acknowledgement_v0.1.up.sql"))
	if !strings.Contains(up, "create table if not exists control_tower.critical_event_acknowledgement") {
		t.Fatal("up migration must create critical_event_acknowledgement table")
	}
	if !strings.Contains(up, "primary key (tenant_id, event_id)") {
		t.Fatal("up migration must define tenant_id + event_id primary key")
	}
	if !strings.Contains(up, "chk_critical_event_ack_event_id_format") {
		t.Fatal("up migration must validate event_id format")
	}
}

func TestMigration000020DownDropsAcknowledgementTable(t *testing.T) {
	t.Parallel()
	down := strings.ToLower(readMigration(t, "000020_create_control_tower_critical_event_acknowledgement_v0.1.down.sql"))
	if !strings.Contains(down, "drop table if exists control_tower.critical_event_acknowledgement") {
		t.Fatal("down migration must drop critical_event_acknowledgement table")
	}
}
