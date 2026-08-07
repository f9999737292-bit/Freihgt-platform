package legacyaggregate

import (
	"fmt"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

// AggregateSummary is the normalized legacy status aggregate used for validation.
type AggregateSummary struct {
	TotalShipments   int64
	CountedShipments int64
	ByStatus         map[string]int64
	Complete         bool
}

// ValidateCompleteLegacyAggregate checks whether a legacy aggregate is safe to use
// as an authoritative shadow baseline or full legacy fallback.
func ValidateCompleteLegacyAggregate(summary AggregateSummary) error {
	if summary.ByStatus == nil {
		return fmt.Errorf("byStatus is required")
	}
	if summary.TotalShipments < 0 || summary.CountedShipments < 0 {
		return fmt.Errorf("negative totals")
	}
	if summary.CountedShipments > summary.TotalShipments {
		return fmt.Errorf("countedShipments exceeds totalShipments")
	}
	if !summary.Complete {
		return fmt.Errorf("aggregate incomplete")
	}
	if summary.TotalShipments != summary.CountedShipments {
		return fmt.Errorf("totalShipments must equal countedShipments for complete aggregate")
	}
	var sum int64
	for status, count := range summary.ByStatus {
		if count < 0 {
			return fmt.Errorf("negative count for status")
		}
		if !controltowerreadmodel.IsKnownShipmentStatus(status) {
			return fmt.Errorf("unknown status in byStatus")
		}
		sum += count
	}
	if sum != summary.CountedShipments {
		return fmt.Errorf("countedShipments mismatch")
	}
	return nil
}

// ValidateAggregateContract performs structural validation without requiring complete=true.
func ValidateAggregateContract(summary AggregateSummary) error {
	if summary.ByStatus == nil {
		return fmt.Errorf("byStatus is required")
	}
	if summary.TotalShipments < 0 || summary.CountedShipments < 0 {
		return fmt.Errorf("negative totals")
	}
	if summary.CountedShipments > summary.TotalShipments {
		return fmt.Errorf("countedShipments exceeds totalShipments")
	}
	var sum int64
	for status, count := range summary.ByStatus {
		if count < 0 {
			return fmt.Errorf("negative count for status")
		}
		if !controltowerreadmodel.IsKnownShipmentStatus(status) {
			return fmt.Errorf("unknown status in byStatus")
		}
		sum += count
	}
	if sum != summary.CountedShipments {
		return fmt.Errorf("countedShipments mismatch")
	}
	if summary.Complete && summary.TotalShipments != summary.CountedShipments {
		return fmt.Errorf("totalShipments must equal countedShipments for complete aggregate")
	}
	return nil
}
