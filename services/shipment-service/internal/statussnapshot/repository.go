package statussnapshot

import "context"

// SnapshotRepository streams authoritative shipment rows in stable tenant_id, shipment_id order.
type SnapshotRepository interface {
	StreamShipmentStatusSnapshot(
		ctx context.Context,
		request SnapshotRequest,
		consume func(ShipmentSnapshotRow) error,
	) (SnapshotStats, error)
}

type NotImplementedRepository struct{}

func (NotImplementedRepository) StreamShipmentStatusSnapshot(
	ctx context.Context,
	request SnapshotRequest,
	consume func(ShipmentSnapshotRow) error,
) (SnapshotStats, error) {
	return SnapshotStats{}, ErrNotImplemented
}
