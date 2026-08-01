package repository

import (
	"strings"
	"testing"
)

func TestStatusSummaryAggregateQueryIsTenantScoped(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(statusSummaryAggregateQuery)
	if !strings.Contains(query, "tenant_id = $1") {
		t.Fatalf("query must filter by tenant_id: %q", statusSummaryAggregateQuery)
	}
}

func TestStatusSummaryAggregateQueryExcludesSoftDeleted(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(statusSummaryAggregateQuery)
	if !strings.Contains(query, "deleted_at is null") {
		t.Fatalf("query must exclude soft-deleted shipments: %q", statusSummaryAggregateQuery)
	}
}

func TestStatusSummaryAggregateQueryGroupsByStatus(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(statusSummaryAggregateQuery)
	if !strings.Contains(query, "group by status") {
		t.Fatalf("query must aggregate by status: %q", statusSummaryAggregateQuery)
	}
}

func TestStatusSummaryAggregateQueryUsesSingleScopedLookup(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(statusSummaryAggregateQuery)
	if strings.Count(query, "from transport.shipments") != 1 {
		t.Fatalf("query must use a single shipments lookup: %q", statusSummaryAggregateQuery)
	}
	if !strings.Contains(query, "where tenant_id = $1") {
		t.Fatalf("query must not perform unscoped shipments lookup: %q", statusSummaryAggregateQuery)
	}
}
