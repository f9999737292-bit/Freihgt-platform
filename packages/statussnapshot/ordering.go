package statussnapshot

import (
	"github.com/google/uuid"
)

type Ordering string

const (
	OrderingTenantIDShipmentID Ordering = "TENANT_ID_SHIPMENT_ID"
)

type ShipmentKey struct {
	TenantID   uuid.UUID
	ShipmentID uuid.UUID
}

// CompareShipmentKeys returns -1 if a < b, 0 if equal, 1 if a > b.
func CompareShipmentKeys(a, b ShipmentKey) int {
	if a.TenantID != b.TenantID {
		if a.TenantID.String() < b.TenantID.String() {
			return -1
		}
		return 1
	}
	if a.ShipmentID == b.ShipmentID {
		return 0
	}
	if a.ShipmentID.String() < b.ShipmentID.String() {
		return -1
	}
	return 1
}

func validateShipmentOrder(previous *ShipmentKey, current ShipmentKey, hasPrevious bool) error {
	if !hasPrevious {
		return nil
	}
	switch CompareShipmentKeys(current, *previous) {
	case -1:
		return &ValidationError{Code: CodeRecordOrderViolation}
	case 0:
		return &ValidationError{Code: CodeDuplicateShipment}
	default:
		return nil
	}
}
