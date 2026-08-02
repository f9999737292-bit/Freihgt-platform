package statussnapshot

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestCompareKeysOrdering(t *testing.T) {
	tenantA, tenantB := uuid.New(), uuid.New()
	if tenantA.String() > tenantB.String() {
		tenantA, tenantB = tenantB, tenantA
	}
	shipA, shipB := uuid.New(), uuid.New()
	if shipA.String() > shipB.String() {
		shipA, shipB = shipB, shipA
	}
	if compareKeys(tenantA, shipB, tenantA, shipA) <= 0 {
		t.Fatal("expected shipB after shipA within tenant")
	}
	if compareKeys(tenantB, shipA, tenantA, shipA) <= 0 {
		t.Fatal("expected tenantA before tenantB")
	}
	if compareKeys(tenantA, shipA, tenantA, shipA) != 0 {
		t.Fatal("expected equal keys")
	}
}

func TestValidateAuthoritativeRowCreatedRejected(t *testing.T) {
	err := validateAuthoritativeRow(scannedRow{
		ShipmentSnapshotRow: ShipmentSnapshotRow{
			TenantID: uuid.New(), ShipmentID: uuid.New(), CurrentStatus: domain.ShipmentStatusCreated,
			AggregateVersion: 1, SourceUpdatedAt: time.Now().UTC(),
		},
		hasHistory: true, historyToStatus: domain.ShipmentStatusCreated, historyVersion: 1,
	})
	if err == nil || ExportErrorCode(err) != CodeUnsupportedShipmentStatus {
		t.Fatalf("expected unsupported status, got %v", err)
	}
}

func TestValidateAuthoritativeRowMissingHistory(t *testing.T) {
	err := validateAuthoritativeRow(scannedRow{
		ShipmentSnapshotRow: ShipmentSnapshotRow{
			TenantID: uuid.New(), ShipmentID: uuid.New(), CurrentStatus: "CARRIER_ASSIGNED",
			AggregateVersion: 1,
		},
		hasHistory: false,
	})
	if err == nil || ExportErrorCode(err) != CodeMissingCanonicalStatusHistory {
		t.Fatalf("expected missing history, got %v", err)
	}
}

func TestValidateAuthoritativeRowStatusMismatch(t *testing.T) {
	err := validateAuthoritativeRow(scannedRow{
		ShipmentSnapshotRow: ShipmentSnapshotRow{
			TenantID: uuid.New(), ShipmentID: uuid.New(), CurrentStatus: "IN_TRANSIT",
			AggregateVersion: 2, SourceUpdatedAt: time.Now().UTC(),
		},
		hasHistory: true, historyToStatus: "DELIVERED", historyVersion: 2,
	})
	if err == nil || ExportErrorCode(err) != CodeAuthoritativeStatusMismatch {
		t.Fatalf("expected mismatch, got %v", err)
	}
}

func TestExportErrorCode(t *testing.T) {
	if ExportErrorCode(newExportError(CodeSourceRowCountMismatch, nil)) != CodeSourceRowCountMismatch {
		t.Fatal("export error code not extracted")
	}
}
