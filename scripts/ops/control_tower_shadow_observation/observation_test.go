package main

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeCommitted(t *testing.T) {
	cases := map[string]string{
		"":    "UNCOMMITTED",
		"-":   "UNCOMMITTED",
		"-1":  "UNCOMMITTED",
		"0":   "0",
		"214": "214",
	}
	for in, want := range cases {
		got := normalizeCommitted(in)
		if got.State != want {
			t.Fatalf("%q => %q, want %q", in, got.State, want)
		}
	}
}

func TestParseRPKGroupDescribe(t *testing.T) {
	sample := `GROUP        control-tower-shipment-status-v1
TOPIC               PARTITION  CURRENT-OFFSET  LOG-START-OFFSET  LOG-END-OFFSET  LAG   MEMBER-ID
shipment.status.v1  0          214             0                 221             7     member
shipment.status.v1  1          -               0                 189             -     member
shipment.status.v1  2          114             0                 125             11    member`
	parts, err := parseRPKGroupDescribe(sample, "shipment.status.v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("expected 3 partitions, got %d", len(parts))
	}
	if parts[0].Committed.State != "214" || parts[0].Lag != 7 {
		t.Fatalf("partition 0: %+v", parts[0])
	}
	if parts[1].Committed.State != "UNCOMMITTED" || parts[1].Lag != -1 {
		t.Fatalf("partition 1 UNCOMMITTED: %+v", parts[1])
	}
}

func TestAssertCommittedUnchanged(t *testing.T) {
	before := []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "5"}, LogEnd: 10, Lag: 5}}
	after := []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "5"}, LogEnd: 10, Lag: 5}}
	if err := assertCommittedUnchanged(before, after); err != nil {
		t.Fatal(err)
	}
	after[0].Committed.State = "6"
	if err := assertCommittedUnchanged(before, after); err == nil {
		t.Fatal("expected error on committed change")
	}
}

func TestAssertCatchUp(t *testing.T) {
	before := []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "214"}, LogEnd: 214, Lag: 0}}
	during := []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "214"}, LogEnd: 221, Lag: 7}}
	after := []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "221"}, LogEnd: 221, Lag: 0}}
	if err := assertCatchUp(before, during, after); err != nil {
		t.Fatal(err)
	}
}

func TestRedactJWT(t *testing.T) {
	if redactJWT("secret-token") != "[REDACTED]" {
		t.Fatal("jwt not redacted")
	}
}

func TestFormatPartitionOffsetsNoUUID(t *testing.T) {
	out := formatPartitionOffsets("Before pause", []PartitionOffset{
		{Partition: 0, Committed: CommittedOffset{State: "5"}, LogEnd: 10, Lag: 5},
	})
	if strings.Contains(out, "tenant") || strings.Contains(out, "Bearer") {
		t.Fatalf("unexpected sensitive content: %s", out)
	}
}

func TestLoadCohortRequiresAlias(t *testing.T) {
	_, err := loadCohort("testdata/invalid_cohort.json")
	if err == nil {
		t.Fatal("expected invalid cohort error")
	}
}

func TestLoadCohortSecretRef(t *testing.T) {
	t.Setenv("CONTROL_TOWER_TENANT_TEST", "00000000-0000-4000-8000-000000000099")
	entries, err := loadCohort("testdata/valid_cohort_secret_ref.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	id, err := entries[0].resolveTenantID()
	if err != nil {
		t.Fatal(err)
	}
	if id != "00000000-0000-4000-8000-000000000099" {
		t.Fatalf("unexpected tenant id: %s", id)
	}
}

func TestLoadCohortSkipsUnapproved(t *testing.T) {
	t.Setenv("CONTROL_TOWER_TENANT_TEST", "00000000-0000-4000-8000-000000000099")
	raw := `{"environment":"staging","tenants":[{"alias":"STG-X","tenantIdFromSecretRef":"CONTROL_TOWER_TENANT_TEST","approved":false}]}`
	path := t.TempDir() + "/cohort.json"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadCohort(path)
	if err == nil {
		t.Fatal("expected no approved tenants error")
	}
}

func TestRedactTenantID(t *testing.T) {
	if redactTenantID("00000000-0000-4000-8000-000000000001") != "[REDACTED]" {
		t.Fatal("tenant id not redacted")
	}
}
