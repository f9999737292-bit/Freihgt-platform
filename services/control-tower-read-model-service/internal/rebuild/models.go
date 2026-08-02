package rebuild

import (
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/statussnapshot"
)

const (
	StateImporting = "IMPORTING"
	StateValidated = "VALIDATED"
	StateFailed    = "FAILED"
)

type Manifest struct {
	SnapshotID    uuid.UUID
	SchemaVersion int
	Scope         statussnapshot.Scope
	TenantID      *uuid.UUID
	StartedAt     time.Time
}

type StageRow struct {
	SnapshotID        uuid.UUID
	TenantID          uuid.UUID
	ShipmentID        uuid.UUID
	CurrentStatus     string
	PreviousStatus    *string
	AggregateVersion  int64
	LastEventID       *uuid.UUID
	LastSourceEventID *uuid.UUID
	SourceUpdatedAt   time.Time
	RecordSequence    int64
}

type ValidationResult struct {
	SnapshotID     uuid.UUID
	ExpectedRows   int64
	TenantCount    int64
	ExpectedSHA256 string
	ActualSHA256   string
}

type JobStatus struct {
	SnapshotID      uuid.UUID
	State           string
	Scope           string
	ExpectedRows    *int64
	ImportedRows    int64
	TenantCount     *int64
	ChecksumMatched bool
	StartedAt       time.Time
	ValidatedAt     *time.Time
	ErrorCode       *string
}

type StatusResponse struct {
	State           string  `json:"state"`
	Scope           string  `json:"scope"`
	ExpectedRows    *int64  `json:"expectedRows"`
	ImportedRows    int64   `json:"importedRows"`
	TenantCount     *int64  `json:"tenantCount"`
	ChecksumMatched bool    `json:"checksumMatched"`
	StartedAt       string  `json:"startedAt"`
	ValidatedAt     *string `json:"validatedAt"`
	ErrorCode       *string `json:"errorCode"`
}

type DryRunReport struct {
	SchemaVersion    int                  `json:"schemaVersion"`
	Scope            statussnapshot.Scope `json:"scope"`
	RowCount         int64                `json:"rowCount"`
	TenantCount      int64                `json:"tenantCount"`
	ChecksumMatched  bool                 `json:"checksumMatched"`
	ValidationResult string               `json:"validationResult"`
}
