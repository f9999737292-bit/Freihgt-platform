package statussnapshot

import (
	"time"

	"github.com/google/uuid"
)

type SnapshotRequest struct {
	Scope    string
	TenantID *uuid.UUID
}

type ShipmentSnapshotRow struct {
	TenantID          uuid.UUID
	ShipmentID        uuid.UUID
	CurrentStatus     string
	PreviousStatus    *string
	AggregateVersion  int64
	LastEventID       *uuid.UUID
	LastSourceEventID *uuid.UUID
	LastEventType     *string
	SourceUpdatedAt   time.Time
}

type SnapshotStats struct {
	RowCount    int64
	TenantCount int64
}
