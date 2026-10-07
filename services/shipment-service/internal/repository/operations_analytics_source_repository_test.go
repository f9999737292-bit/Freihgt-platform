package repository

import (
	"strings"
	"testing"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestOperationsAnalyticsSourceStatementIsOneRead(t *testing.T) {
	sql := strings.ToLower(OperationsAnalyticsSourceStatement)
	if strings.Contains(sql, ";") {
		t.Fatal("statement must be a single query")
	}
	for _, forbidden := range []string{"insert ", "update ", "delete ", "merge ", "alter ", "create ", "drop "} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("write or DDL keyword %q", forbidden)
		}
	}
	for _, required := range []string{
		"from transport.shipments",
		"where tenant_id = $1",
		"deleted_at is null",
		"planned_delivery_at is not null",
		"actual_delivery_at is not null",
		"actual_delivery_at <= planned_delivery_at",
		"from transport.delivery_disposition_cases",
		"where operating_tenant_id = $1",
		"disposition_type = '" + strings.ToLower(domain.DispositionReturnToOrigin) + "'",
		"disposition_type = '" + strings.ToLower(domain.DispositionRedirect) + "'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("missing %q", required)
		}
	}
	for _, absent := range []string{
		"ops_shipments_total",
		"ops_on_time_delivery",
		"ops_return_cases",
		"ops_redirect_cases",
		"definitionversion",
		"otif",
		"time.now",
		"max(updated_at)",
		"max(created_at)",
	} {
		if strings.Contains(sql, absent) {
			t.Fatalf("source statement contains %q", absent)
		}
	}
	if strings.Contains(sql, "hold_pending_disposition") {
		t.Fatal("hold cases must not be selected")
	}
}
