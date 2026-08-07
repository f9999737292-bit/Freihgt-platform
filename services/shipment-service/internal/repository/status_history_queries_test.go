package repository

import (
	"strings"
	"testing"
)

func TestInsertStatusHistoryQueryUsesTenantParameter(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(insertStatusHistoryQuery)
	if !strings.Contains(query, "insert into transport.shipment_status_history") {
		t.Fatalf("unexpected insert query: %q", insertStatusHistoryQuery)
	}
	if !strings.Contains(query, "tenant_id") {
		t.Fatal("insert must include tenant_id column")
	}
}

func TestListStatusHistoryQueryIsTenantScoped(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(listStatusHistoryQuery)
	if !strings.Contains(query, "where tenant_id = $1 and shipment_id = $2") {
		t.Fatalf("list query must filter by tenant and shipment: %q", listStatusHistoryQuery)
	}
}

func TestCountStatusHistoryQueryIsTenantScoped(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(countStatusHistoryQuery)
	if !strings.Contains(query, "where tenant_id = $1 and shipment_id = $2") {
		t.Fatalf("count query must filter by tenant and shipment: %q", countStatusHistoryQuery)
	}
}

func TestHasInitialStatusHistoryQueryChecksNullFromStatus(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(hasInitialStatusHistoryQuery)
	if !strings.Contains(query, "from_status is null") {
		t.Fatalf("initial history check must look for from_status IS NULL: %q", hasInitialStatusHistoryQuery)
	}
	if !strings.Contains(query, "tenant_id = $1 and shipment_id = $2") {
		t.Fatal("initial history check must be tenant-scoped")
	}
}
