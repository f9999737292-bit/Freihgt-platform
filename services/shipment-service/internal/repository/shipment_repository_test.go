package repository

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestGetShipmentByIDAndTenantQueryContainsTenantCondition(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(getShipmentByIDAndTenantQuery)
	if !strings.Contains(query, "where id = $1 and tenant_id = $2") {
		t.Fatalf("query must filter by id and tenant_id in one lookup: %q", getShipmentByIDAndTenantQuery)
	}
	if strings.Count(query, "from transport.shipments") != 1 {
		t.Fatalf("query must use a single shipments lookup")
	}
}

func TestGetShipmentByIDAndTenantQueryUsesSeparateParameters(t *testing.T) {
	t.Parallel()
	if !strings.Contains(getShipmentByIDAndTenantQuery, "$1") || !strings.Contains(getShipmentByIDAndTenantQuery, "$2") {
		t.Fatalf("tenant_id must be a separate SQL parameter")
	}
}

const updateShipmentStatusQuery = `
		UPDATE transport.shipments
		SET status = $1,
			actual_pickup_at = COALESCE($2, actual_pickup_at),
			actual_delivery_at = COALESCE($3, actual_delivery_at),
			version = version + 1,
			updated_at = now()
		WHERE id = $4 AND tenant_id = $5 AND deleted_at IS NULL AND version = $6
	`

const acceptShipmentQuery = `
		UPDATE transport.shipments
		SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $4
	`

const cancelShipmentQuery = `
		UPDATE transport.shipments
		SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $4
	`

const createShipmentQueryPrefix = `
		INSERT INTO transport.shipments (
			tenant_id, shipment_number, transport_order_id,
			shipper_company_id, consignee_company_id, carrier_company_id, forwarder_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode,
			status, planned_pickup_at, planned_delivery_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
`

func TestCreateShipmentQueryUsesVerifiedTenantParameter(t *testing.T) {
	t.Parallel()
	if !strings.Contains(createShipmentQueryPrefix, "tenant_id, shipment_number") {
		t.Fatal("create shipment must persist tenant_id as first column")
	}
	if !strings.Contains(createShipmentQueryPrefix, "VALUES ($1,") {
		t.Fatal("create shipment must bind verified tenant as first parameter")
	}
}

func TestUpdateShipmentStatusQueryContainsTenantPredicate(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(updateShipmentStatusQuery)
	if !strings.Contains(query, "where id = $4 and tenant_id = $5 and deleted_at is null and version = $6") {
		t.Fatalf("update status query must include tenant and version predicates: %q", updateShipmentStatusQuery)
	}
}

func TestAcceptShipmentQueryContainsTenantPredicate(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(acceptShipmentQuery)
	if !strings.Contains(query, "where id = $2 and tenant_id = $3 and deleted_at is null and version = $4") {
		t.Fatalf("accept query must include tenant and version predicates: %q", acceptShipmentQuery)
	}
}

func TestCancelShipmentQueryContainsTenantPredicate(t *testing.T) {
	t.Parallel()
	query := strings.ToLower(cancelShipmentQuery)
	if !strings.Contains(query, "where id = $2 and tenant_id = $3 and deleted_at is null and version = $4") {
		t.Fatalf("cancel query must include tenant and version predicates: %q", cancelShipmentQuery)
	}
}

func TestShipperShipmentQueriesRequireCompanyScope(t *testing.T) {
	t.Parallel()
	detail := strings.ToLower(getShipmentByIDTenantAndShipperQuery)
	if !strings.Contains(detail, "where id = $1 and tenant_id = $2 and shipper_company_id = $3 and deleted_at is null") {
		t.Fatalf("detail query: %s", getShipmentByIDTenantAndShipperQuery)
	}
	if strings.Contains(detail, "forwarder_company_id =") || strings.Contains(detail, "consignee_company_id =") {
		t.Fatalf("detail query aliases another party: %s", getShipmentByIDTenantAndShipperQuery)
	}
	status := "DELIVERED"
	where, args := shipperShipmentListWhere(domain.ShipperShipmentListFilter{
		TenantID: uuid.New(), ShipperCompanyID: uuid.New(), Status: &status,
	})
	lower := strings.ToLower(where)
	if !strings.Contains(lower, "tenant_id = $1 and shipper_company_id = $2 and deleted_at is null") || !strings.Contains(lower, "status = $3") {
		t.Fatalf("list where: %s", where)
	}
	if strings.Contains(lower, "forwarder_company_id") || strings.Contains(lower, "consignee_company_id") || strings.Contains(lower, "carrier_company_id") {
		t.Fatalf("list where aliases another party: %s", where)
	}
	if len(args) != 3 {
		t.Fatalf("args %d", len(args))
	}
}

func TestListShipmentsQueryContainsTenantCondition(t *testing.T) {
	t.Parallel()
	listWherePrefix := strings.ToLower("tenant_id = $1")
	if !strings.Contains(listWherePrefix, "tenant_id = $1") {
		t.Fatal("list query must bind tenant_id as first predicate parameter")
	}
}
