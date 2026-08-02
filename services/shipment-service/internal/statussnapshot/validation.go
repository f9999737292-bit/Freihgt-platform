package statussnapshot

import (
	"errors"

	"github.com/freight-platform/statussnapshot"
)

var ErrUnknownStatus = errors.New("unknown shipment status")

func ValidateSnapshotRow(row ShipmentSnapshotRow) error {
	if !statussnapshot.IsKnownStatus(row.CurrentStatus) {
		return ErrUnknownStatus
	}
	if row.PreviousStatus != nil && !statussnapshot.IsKnownStatus(*row.PreviousStatus) {
		return ErrUnknownStatus
	}
	if row.AggregateVersion < 1 {
		return &statussnapshot.ValidationError{Code: statussnapshot.CodeInvalidAggregateVersion}
	}
	return nil
}
