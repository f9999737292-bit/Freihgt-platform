package statussnapshot

import (
	"strings"
	"testing"
)

func TestTenantSnapshotQueryFiltersHistoryInCTE(t *testing.T) {
	if !strings.Contains(snapshotStreamQueryTenant, "WHERE h.tenant_id = $1") {
		t.Fatal("tenant snapshot query must filter history inside CTE")
	}
	if !strings.Contains(snapshotStreamQueryTenant, "WHERE s.deleted_at IS NULL") ||
		!strings.Contains(snapshotStreamQueryTenant, "AND s.tenant_id = $1") {
		t.Fatal("tenant snapshot query must filter shipments by tenant and deleted_at")
	}
}

func TestAllSnapshotQueryDoesNotFilterHistoryByTenant(t *testing.T) {
	if strings.Contains(snapshotStreamQueryAll, "WHERE h.tenant_id") {
		t.Fatal("all-scope query must not filter history CTE by tenant")
	}
}
